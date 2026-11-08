package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Nem-git/ABCMovies/internal/config"
	"github.com/Nem-git/ABCMovies/internal/web"
)

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
