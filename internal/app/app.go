// Package app wires the ABCMovies deployable units together.
// Three independent units exist by design (see decisions/tech/0010):
//   - Core: gRPC-only service backed by SQLite. No HTTP, no cookies.
//   - API:  HTTP edge; the single place session/cookie semantics live.
//   - Web:  SSR HTML frontend; calls the API server-side; no session logic.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
	"github.com/Nem-git/ABCMovies/internal/api"
	"github.com/Nem-git/ABCMovies/internal/config"
	"github.com/Nem-git/ABCMovies/internal/core"
	"github.com/Nem-git/ABCMovies/internal/store"
	"github.com/Nem-git/ABCMovies/internal/web"
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

// StartCore opens the store and serves the identity gRPC service.
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
	lis, addr, err := config.Listen(bind, cfg.Port)
	if err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("listen grpc: %w", err)
	}
	grpcSrv := grpc.NewServer()
	identityv1.RegisterIdentityServiceServer(grpcSrv, core.NewIdentityService(st))
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

// APIConfig holds configuration for the HTTP API frontend.
type APIConfig struct {
	Bind string
	Port int
	// CoreGRPCAddr is the address of the core, e.g. "127.0.0.1:9876".
	CoreGRPCAddr string
}

// API is a running HTTP API frontend.
type API struct {
	HTTPAddr string

	conn     *grpc.ClientConn
	httpSrv  *http.Server
	listener net.Listener
}

// StartAPI connects to the core over gRPC and serves the HTTP API.
// Port 0 selects an ephemeral port (useful in tests).
func StartAPI(cfg APIConfig) (*API, error) {
	bind := cfg.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	if cfg.CoreGRPCAddr == "" {
		return nil, errors.New("CoreGRPCAddr is required")
	}
	conn, err := grpc.NewClient(cfg.CoreGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial core: %w", err)
	}
	handler := api.NewServer(identityv1.NewIdentityServiceClient(conn)).Handler()
	lis, addr, err := config.Listen(bind, cfg.Port)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("listen http: %w", err)
	}
	srv := config.NewHTTPServer(handler)
	go func() { _ = srv.Serve(lis) }()
	return &API{HTTPAddr: addr, conn: conn, httpSrv: srv, listener: lis}, nil
}

// Close shuts down the API frontend.
func (a *API) Close() {
	if a.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.httpSrv.Shutdown(ctx)
	}
	if a.listener != nil {
		_ = a.listener.Close()
	}
	if a.conn != nil {
		_ = a.conn.Close()
	}
}

// WebConfig holds configuration for the SSR web frontend.
type WebConfig struct {
	Bind string
	Port int
	// APIBaseURL is the base URL of the HTTP API, e.g. "http://127.0.0.1:8096".
	APIBaseURL string
}

// Web is a running SSR web frontend.
type Web struct {
	HTTPAddr string

	httpSrv  *http.Server
	listener net.Listener
}

// StartWeb serves the SSR web frontend against the given HTTP API.
// Port 0 selects an ephemeral port (useful in tests).
func StartWeb(cfg WebConfig) (*Web, error) {
	bind := cfg.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	httpServer, err := web.NewServer(cfg.APIBaseURL)
	if err != nil {
		return nil, err
	}
	lis, addr, err := config.Listen(bind, cfg.Port)
	if err != nil {
		return nil, fmt.Errorf("listen http: %w", err)
	}
	srv := config.NewHTTPServer(httpServer.Handler())
	go func() { _ = srv.Serve(lis) }()
	return &Web{HTTPAddr: addr, httpSrv: srv, listener: lis}, nil
}

// Close shuts down the web frontend.
func (w *Web) Close() {
	if w.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.httpSrv.Shutdown(ctx)
	}
	if w.listener != nil {
		_ = w.listener.Close()
	}
}
