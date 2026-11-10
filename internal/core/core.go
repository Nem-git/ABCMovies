// Package core is the public composition wrapper for the gRPC-only ABCMovies
// core. The implementation lives in internal/core/internal.
package core

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"google.golang.org/grpc"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
	sourcev1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/source/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/service"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
	"github.com/Nem-git/ABCMovies/internal/platform"
)

// CoreConfig holds configuration for the gRPC core.
type CoreConfig struct {
	Bind string
	Port int
	DB   string
}

// Core is a running core (store + gRPC listener). It serves no HTTP.
type Core struct {
	GRPCAddr string

	store    *store.Store
	grpcSrv  *grpc.Server
	listener net.Listener
}

// StartCore opens the store and serves the identity, source, and library gRPC services.
// Port 0 selects an ephemeral port (useful in tests).
func StartCore(cfg CoreConfig) (*Core, error) {
	bind := cfg.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	if cfg.DB == "" {
		return nil, errors.New("DB path is required")
	}
	if dir := filepath.Dir(cfg.DB); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	st, err := store.Open(cfg.DB)
	if err != nil {
		return nil, err
	}
	lis, addr, err := platform.Listen(bind, cfg.Port)
	if err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("listen grpc: %w", err)
	}
	grpcSrv := grpc.NewServer()
	identityv1.RegisterIdentityServiceServer(grpcSrv, service.NewIdentityService(st))
	sv := service.NewSessionState()
	sourcev1.RegisterSourceServiceServer(grpcSrv, service.NewSourceService(st, sv))
	libraryv1.RegisterLibraryServiceServer(grpcSrv, service.NewLibraryService(st, sv))
	go func() { _ = grpcSrv.Serve(lis) }()
	return &Core{GRPCAddr: addr, store: st, grpcSrv: grpcSrv, listener: lis}, nil
}

// Close shuts down the core.
func (c *Core) Close() {
	if c.grpcSrv != nil {
		c.grpcSrv.Stop()
	}
	if c.listener != nil {
		_ = c.listener.Close()
	}
	if c.store != nil {
		_ = c.store.Close()
	}
}
