package apiserver

import (
	"context"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/slothealth"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	users := store.NewInMemory()
	srv := NewServer(Settings{Stores: config.Stores{Users: users}})
	srv.SetSlotHealth(sh)
	ctx := WithUserID(context.Background(), "user:op")
	if err := auth.NewStoreUserStore(users).PutUser("op", &auth.UserData{Role: auth.RoleOperator}); err != nil {
		t.Fatal(err)
	}
	resp, err := srv.ListSlotHealth(ctx, &apiv1.ListSlotHealthRequest{})
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

// TestListSlotHealth_RequiresOperator pins the D4 gate: without a principal
// the call is Unauthenticated, a regular user is PermissionDenied, and only
// an operator gets the records.
func TestListSlotHealth_RequiresOperator(t *testing.T) {
	users := store.NewInMemory()
	srv := NewServer(Settings{Stores: config.Stores{Users: users}})
	srv.SetSlotHealth(mustSlotHealth(t))

	if _, err := srv.ListSlotHealth(context.Background(), &apiv1.ListSlotHealthRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("no principal = %v, want Unauthenticated", err)
	}

	if err := auth.NewStoreUserStore(users).PutUser("alice", &auth.UserData{Role: auth.RoleUser}); err != nil {
		t.Fatal(err)
	}
	userCtx := WithUserID(context.Background(), "user:alice")
	if _, err := srv.ListSlotHealth(userCtx, &apiv1.ListSlotHealthRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("regular user = %v, want PermissionDenied", err)
	}

	// The gate runs even where there is nothing to return: an unwired server
	// must not hand a non-operator an OK for absent data.
	if err := auth.NewStoreUserStore(users).PutUser("op", &auth.UserData{Role: auth.RoleOperator}); err != nil {
		t.Fatal(err)
	}
	opCtx := WithUserID(context.Background(), "user:op")
	if _, err := srv.ListSlotHealth(opCtx, &apiv1.ListSlotHealthRequest{}); err != nil {
		t.Fatalf("operator on unwired slot health = %v, want nil error", err)
	}
}

func mustSlotHealth(t *testing.T) *slothealth.Provider {
	t.Helper()
	sh, err := slothealth.New(store.NewInMemory())
	if err != nil {
		t.Fatal(err)
	}
	return sh
}
