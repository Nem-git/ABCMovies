package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/scheduler"

	"github.com/nem-git/abcmovies/core/app"
)

// instanceConfigPath is the documented instance configuration location
// (ENVIRONMENT.md §6); a missing file means defaults.
const instanceConfigPath = "config/instance.yaml"

func main() {
	logger := slog.Default()
	ctx := context.Background()

	stack, err := app.Build(instanceConfigPath, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: %v\n", err)
		os.Exit(1)
	}
	defer stack.Close()

	slots, err := stack.BuildSlots(ctx, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: %v\n", err)
		os.Exit(1)
	}

	sched := scheduler.New(0, logger)
	for _, j := range slots.Jobs {
		sched.Register(j)
	}
	go sched.Run(ctx)

	unary, stream := stack.AuthInterceptors()
	gs := grpc.NewServer(
		grpc.UnaryInterceptor(unary),
		grpc.StreamInterceptor(stream),
	)
	apiv1.RegisterCoreServiceServer(gs, stack.Service())

	lis, err := net.Listen("tcp", stack.BindAddress())
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: listen %s: %v\n", stack.BindAddress(), err)
		os.Exit(1)
	}

	go func() {
		logger.Info("listening", "address", stack.BindAddress())
		if err := gs.Serve(lis); err != nil {
			fmt.Fprintf(os.Stderr, "abcmovies: serve: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")
	gs.GracefulStop()
}
