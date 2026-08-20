package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/builtin"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/registry"
)

func main() {
	logger := slog.Default()
	ctx := context.Background()

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: config: %v\n", err)
		os.Exit(1)
	}

	// Build store backends from config.
	stores, err := config.BuildStores(ctx, cfg, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: stores: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = stores.Cache.Close()
		_ = stores.Vault.Close()
		_ = stores.WatchHistory.Close()
		_ = stores.Jobs.Close()
		_ = stores.Sessions.Close()
	}()

	// Set up auth system.
	userStore := auth.NewMemoryUserStore()
	tokenStore, dekCache := config.BuildAuth(stores.Sessions)
	authenticator := auth.NewPasswordAuthenticator(userStore)
	tokenTTL := config.ParseTokenTTL(cfg.Auth.TokenTTL)
	session := auth.NewInMemorySession(tokenStore, dekCache, tokenTTL)

	r := registry.NewInProcess()
	defer r.Close()

	caps, err := r.Admit("builtin", builtin.New())
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("registry up; built-in slot admitted:")
	for _, c := range caps {
		fmt.Printf("  %s v%d\n", c.Name, c.Version)
	}

	bus := apiserver.NewInMemoryBus()
	defer bus.Close()

	srv := apiserver.NewServer(bus, stores, authenticator, session)
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
		fmt.Printf("listening on %s\n", cfg.Core.API.Bind)
		if err := gs.Serve(lis); err != nil {
			fmt.Fprintf(os.Stderr, "abcmovies: serve: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nshutting down...")
	gs.GracefulStop()
}
