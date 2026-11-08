// Command abcmovies is the ABCMovies self-hosted media hub.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/Nem-git/ABCMovies/internal/app"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envPort(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	cfg := app.Config{
		Bind:     env("BIND_ADDRESS", "127.0.0.1"),
		HTTPPort: envPort("HTTP_PORT", 8096),
		GRPCPort: envPort("GRPC_PORT", 9876),
		DBPath:   env("ABCMOVIES_DB", "data/abcmovies.db"),
	}

	ctx := context.Background()
	a, err := app.Start(ctx, cfg)
	if err != nil {
		slog.Error("start", "err", err)
		os.Exit(1)
	}
	defer a.Close()

	slog.Info("abcmovies running", "http", a.HTTPURL, "grpc", a.GRPCAddr)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}