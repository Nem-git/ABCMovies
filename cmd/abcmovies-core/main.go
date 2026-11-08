// Command abcmovies-core is the gRPC-only ABCMovies core: identity, store,
// and sessions. It serves no HTTP and is designed to be deployable on its
// own (see decisions/tech/0010).
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
	cfg := app.CoreConfig{
		Bind: config.Env("CORE_BIND_ADDRESS", "127.0.0.1"),
		Port: config.EnvPort("CORE_GRPC_PORT", 9876),
		DB:   config.Env("ABCMOVIES_DB", "data/abcmovies.db"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	core, err := app.StartCore(cfg)
	if err != nil {
		slog.Error("start core", "err", err)
		os.Exit(1)
	}
	slog.Info("abcmovies core running (gRPC only)", "grpc", core.GRPCAddr)

	<-ctx.Done()
	core.Close()
}
