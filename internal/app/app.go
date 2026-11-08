// Package app wires the store, core identity service (gRPC) and web
// frontend (HTTP) together.
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

	"github.com/Nem-git/ABCMovies/internal/core"
	"github.com/Nem-git/ABCMovies/internal/store"
	"github.com/Nem-git/ABCMovies/internal/web"
	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
)

// Config holds runtime configuration for the app.
type Config struct {
	// Bind is the address both listeners bind to, e.g. 127.0.0.1.
	Bind string
	// HTTPPort is the port for the web frontend listener (0 = ephemeral).
	HTTPPort int
	// GRPCPort is the port for the core gRPC listener (0 = ephemeral).
	GRPCPort int
	// DBPath is the path to the SQLite database file.
	DBPath string
}

// App bundles the running servers and their addresses.
type App struct {
	HTTPURL  string
	GRPCAddr string

	store    *store.Store
	grpcSrv  *grpc.Server
	httpSrv  *http.Server
	conn     *grpc.ClientConn
}

// Start starts both listeners and returns the addresses in use. It is
// suitable for tests (pass port 0) and production.
func Start(ctx context.Context, cfg Config) (*App, error) {
	bind := cfg.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	if cfg.DBPath == "" {
		return nil, errors.New("DBPath is required")
	}

	if dir := filepath.Dir(cfg.DBPath); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	grpcAddr := fmt.Sprintf("%s:%d", bind, cfg.GRPCPort)
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("listen grpc: %w", err)
	}
	grpcAddr = grpcLis.Addr().String()

	grpcSrv := grpc.NewServer()
	identityv1.RegisterIdentityServiceServer(grpcSrv, core.NewIdentityService(st))
	go func() { _ = grpcSrv.Serve(grpcLis) }()

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		grpcSrv.Stop()
		st.Close()
		return nil, fmt.Errorf("dial core: %w", err)
	}

	httpHandler := web.NewServer(identityv1.NewIdentityServiceClient(conn)).Handler()
	httpAddr := fmt.Sprintf("%s:%d", bind, cfg.HTTPPort)
	httpLis, err := net.Listen("tcp", httpAddr)
	if err != nil {
		grpcSrv.Stop()
		st.Close()
		return nil, fmt.Errorf("listen http: %w", err)
	}
	httpAddr = httpLis.Addr().String()
	httpSrv := &http.Server{Handler: httpHandler, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = httpSrv.Serve(httpLis) }()

	return &App{
		HTTPURL:  "http://" + httpAddr,
		GRPCAddr: grpcAddr,
		store:    st,
		grpcSrv:  grpcSrv,
		httpSrv:  httpSrv,
		conn:     conn,
	}, nil
}

// Close shuts down the listeners and closes the store.
func (a *App) Close() {
	if a.conn != nil {
		_ = a.conn.Close()
	}
	if a.httpSrv != nil {
		_ = a.httpSrv.Shutdown(context.Background())
	}
	if a.grpcSrv != nil {
		a.grpcSrv.Stop()
	}
	if a.store != nil {
		_ = a.store.Close()
	}
}