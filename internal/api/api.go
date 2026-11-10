// Package api is the public composition wrapper for the ABCMovies HTTP API
// frontend. The implementation lives in internal/api/internal.
package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
	sourcev1  "github.com/Nem-git/ABCMovies/gen/go/abcmovies/source/v1"
	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
	"github.com/Nem-git/ABCMovies/internal/api/internal/server"
	"github.com/Nem-git/ABCMovies/internal/platform"
)

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
	handler := server.NewServer(identityv1.NewIdentityServiceClient(conn), sourcev1.NewSourceServiceClient(conn), libraryv1.NewLibraryServiceClient(conn)).Handler()
	lis, addr, err := platform.Listen(bind, cfg.Port)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("listen http: %w", err)
	}
	srv := platform.NewHTTPServer(handler)
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
