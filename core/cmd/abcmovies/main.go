package main

import (
	"context"
	"crypto/cipher"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/builtin"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/scheduler"

	"github.com/nem-git/abcmovies/core/app"
)

// instanceConfigPath is the documented instance configuration location
// (ENVIRONMENT.md §6); a missing file means defaults.
const instanceConfigPath = "config/instance.yaml"

func main() {
	logger := slog.Default()
	ctx := context.Background()

	cfg, err := config.Load(instanceConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: config: %v\n", err)
		os.Exit(1)
	}

	stores, err := config.BuildStores(ctx, cfg, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: stores: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = config.CloseStores(stores) }()

	var vaultAEAD cipher.AEAD
	if cfg.Auth.DEKCache == "encrypted-store" {
		vaultAEAD, err = config.VaultAEAD(cfg, logger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "abcmovies: dek-cache: %v\n", err)
			os.Exit(1)
		}
	}
	userStore, tokenStore, dekCache, err := config.BuildAuth(stores.Users, stores.Sessions, cfg.Auth.DEKCache, vaultAEAD)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: auth: %v\n", err)
		os.Exit(1)
	}
	composite, err := config.BuildAuthenticator(cfg.Auth.Methods, userStore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: auth: %v\n", err)
		os.Exit(1)
	}
	session := config.BuildSession(tokenStore, dekCache, config.ParseTokenTTL(cfg.Auth.TokenTTL))

	r := registry.NewInProcess()
	defer r.Close()

	if cfg.Slots.Builtin.Enabled {
		if _, err := r.Admit("builtin", builtin.New()); err != nil {
			fmt.Fprintf(os.Stderr, "abcmovies: %v\n", err)
			os.Exit(1)
		}
		logger.Info("built-in slot admitted", "slot", "builtin")
	}

	sched := scheduler.New(0, logger)

	slots, err := app.ComposeSlots(ctx, cfg.Slots, cfg.Enrichment, r, stores.SourceCache, stores.MetadataCache, stores.Vault, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: %v\n", err)
		os.Exit(1)
	}
	defer slots.Bus.Close()

	for _, j := range slots.Jobs {
		sched.Register(j)
	}
	go sched.Run(ctx)

	srv := apiserver.NewServer(slots.Bus, stores, composite, session)
	gs := grpc.NewServer(
		grpc.UnaryInterceptor(apiserver.AuthUnaryInterceptor(session)),
		grpc.StreamInterceptor(apiserver.AuthStreamInterceptor(session)),
	)
	apiv1.RegisterCoreServiceServer(gs, srv)

	lis, err := net.Listen("tcp", cfg.Core.API.Bind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: listen %s: %v\n", cfg.Core.API.Bind, err)
		os.Exit(1)
	}

	go func() {
		logger.Info("listening", "address", cfg.Core.API.Bind)
		if err := gs.Serve(lis); err != nil {
			fmt.Fprintf(os.Stderr, "abcmovies: serve: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")
	gs.GracefulStop()
}
