// Command web is the reference web frontend: it boots the core in-process
// and serves the browser page and the gRPC-Web API on one port.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nem-git/abcmovies/frontends/web/serving"
)

func main() {
	logger := slog.Default()

	srv, err := serving.New("", logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "web: %v\n", err)
		os.Exit(1)
	}
	defer srv.Close()

	addr := srv.BindAddress()
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		fmt.Printf("web frontend listening on %s\n", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "web: serve: %v\n", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nshutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "web: shutdown: %v\n", err)
	}
}
