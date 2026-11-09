// Command abcmovies-api is the ABCMovies HTTP API frontend: a JSON API with
// the single HTTP session/cookie edge. It serves browser SSR pages' same-origin
// /api/* traffic (via abcmovies-web's proxy) and external frontends. It dials
// the gRPC core and owns all session semantics; the core never sees cookies.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nem-git/ABCMovies/internal/app"
	"github.com/Nem-git/ABCMovies/internal/config"
)

func main() {
	cfg := app.APIConfig{
		Bind:         config.Env("API_BIND_ADDRESS", "127.0.0.1"),
		Port:         config.EnvPort("API_HTTP_PORT", 8098),
		CoreGRPCAddr: config.Env("CORE_GRPC_ADDR", "127.0.0.1:9876"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	api, err := app.StartAPI(cfg)
	if err != nil {
		slog.Error("start api", "err", err)
		os.Exit(1)
	}
	slog.Info("abcmovies api running", "http", "http://"+api.HTTPAddr, "core", cfg.CoreGRPCAddr)

	<-ctx.Done()
	api.Close()
}
