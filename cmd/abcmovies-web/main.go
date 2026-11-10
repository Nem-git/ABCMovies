// Command abcmovies-web is the SSR web frontend: server-rendered HTML pages
// (stdlib html/template with embedded files) plus a same-origin /api/* proxy
// to the ABCMovies HTTP API. It dials no core and owns no session logic.
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
	cfg := app.WebConfig{
		Bind:       config.Env("WEB_BIND_ADDRESS", "127.0.0.1"),
		Port:       config.EnvPort("WEB_HTTP_PORT", 8097),
		APIBaseURL: config.Env("API_HTTP_ADDR", "http://127.0.0.1:8098"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	web, err := app.StartWeb(cfg)
	if err != nil {
		slog.Error("start web", "err", err)
		os.Exit(1)
	}
	slog.Info("abcmovies web running", "http", "http://"+web.HTTPAddr, "api", cfg.APIBaseURL)

	<-ctx.Done()
	web.Close()
}
