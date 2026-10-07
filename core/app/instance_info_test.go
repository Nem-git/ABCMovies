package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

// The composed stack publishes the configured liveness cadence, not a
// hardcoded one: the operator's delivery.heartbeat.interval reaches
// GetInstanceInfo through the full composition (config load → delivery
// wiring → API surface), so a client always learns the value the engine
// actually enforces (PLAN.md §9.1, TECHNICAL-DECISIONS.md §1.43).
func TestComposedStackPublishesConfiguredHeartbeatInterval(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
core:
  api:
    bind: "127.0.0.1:0"
delivery:
  heartbeat:
    interval: 45s
    grace: 2m
  session-ttl: 12h
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stack, err := Build(configPath, slog.Default())
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	defer stack.Close()
	if _, err := stack.BuildSlots(context.Background(), slog.Default()); err != nil {
		t.Fatalf("BuildSlots: %v", err)
	}
	srv, ok := stack.Service().(*apiserver.Server)
	if !ok {
		t.Fatal("service is not a concrete *apiserver.Server")
	}

	resp, err := srv.GetInstanceInfo(context.Background(), &apiv1.GetInstanceInfoRequest{})
	if err != nil {
		t.Fatalf("GetInstanceInfo: %v", err)
	}
	if got := resp.GetHeartbeatInterval().AsDuration(); got != 45*time.Second {
		t.Fatalf("published heartbeat interval = %v, want configured 45s", got)
	}
	if got := resp.GetAuthMethods(); len(got) != 1 || got[0] != "password" {
		t.Fatalf("auth methods = %v, want [password]", got)
	}
}

// Without delivery config the composed stack publishes the shipped default
// (30s, TECHNICAL-DECISIONS.md §1.14).
func TestComposedStackPublishesDefaultHeartbeatInterval(t *testing.T) {
	stack, err := Build("", slog.Default())
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	defer stack.Close()
	if _, err := stack.BuildSlots(context.Background(), slog.Default()); err != nil {
		t.Fatalf("BuildSlots: %v", err)
	}
	srv, ok := stack.Service().(*apiserver.Server)
	if !ok {
		t.Fatal("service is not a concrete *apiserver.Server")
	}
	resp, err := srv.GetInstanceInfo(context.Background(), &apiv1.GetInstanceInfoRequest{})
	if err != nil {
		t.Fatalf("GetInstanceInfo: %v", err)
	}
	if got := resp.GetHeartbeatInterval().AsDuration(); got != 30*time.Second {
		t.Fatalf("published heartbeat interval = %v, want shipped default 30s", got)
	}
}
