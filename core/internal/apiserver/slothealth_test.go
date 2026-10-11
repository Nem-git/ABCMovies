package apiserver

import (
	"context"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/slothealth"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func TestListSlotHealth(t *testing.T) {
	backend := store.NewInMemory()
	sh, err := slothealth.New(backend)
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.RecordProbeSuccess(context.Background(), "jellyfin-1"); err != nil {
		t.Fatal(err)
	}
	if err := sh.RecordProbeFailure(context.Background(), "jellyfin-1", "NETWORK", "down"); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Settings{})
	srv.SetSlotHealth(sh)
	resp, err := srv.ListSlotHealth(context.Background(), &apiv1.ListSlotHealthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Slots) != 1 {
		t.Fatalf("want 1 slot, got %d", len(resp.Slots))
	}
	if resp.Slots[0].Healthy {
		t.Errorf("want healthy false after failure")
	}
	if resp.Slots[0].FailureCount != 1 {
		t.Errorf("want failure_count 1, got %d", resp.Slots[0].FailureCount)
	}
}
