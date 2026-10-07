// End-to-end of the user-triggered refresh RPC: arguments validated, reach
// gate enforced, counts mapped back to the client.
package m7_test

import (
	"context"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type reachableLibrary struct {
	library.Service
}

func (r *reachableLibrary) ReachAuthorized(accountID, userID string) (library.Reach, bool) {
	if accountID == "locked" {
		return library.Reach{}, false
	}
	return library.Reach{AccountID: accountID, Visibility: accounts.VisibilityPublic}, true
}

func (r *reachableLibrary) RefreshAvailability(context.Context, string, []string) (sourcecache.Stats, error) {
	return sourcecache.Stats{Items: 2, Removed: 1}, nil
}

func newRefreshServer(t *testing.T) *apiserver.Server {
	t.Helper()
	srv := apiserver.NewServer(apiserver.NewInMemoryBus(), config.Stores{Jobs: store.NewInMemory(), Vault: store.NewInMemory()}, nil, nil)
	srv.SetLibrary(&reachableLibrary{})
	return srv
}

func TestRefreshAvailabilityRPC(t *testing.T) {
	srv := newRefreshServer(t)
	resp, err := srv.RefreshAvailability(context.Background(), &apiv1.RefreshAvailabilityRequest{
		AccountId: "acct", NativeIds: []string{"movie:tt1", "movie:tt2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetPresent() != 2 || resp.GetRemoved() != 1 {
		t.Fatalf("resp = %v", resp)
	}
}

func TestRefreshAvailabilityValidatesArgs(t *testing.T) {
	srv := newRefreshServer(t)
	if _, err := srv.RefreshAvailability(context.Background(), &apiv1.RefreshAvailabilityRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty request = %v, want InvalidArgument", err)
	}
	if _, err := srv.RefreshAvailability(context.Background(), &apiv1.RefreshAvailabilityRequest{AccountId: "a"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("no native_ids = %v, want InvalidArgument", err)
	}
}

func TestRefreshAvailabilityReachGate(t *testing.T) {
	srv := newRefreshServer(t)
	if _, err := srv.RefreshAvailability(context.Background(), &apiv1.RefreshAvailabilityRequest{
		AccountId: "locked", NativeIds: []string{"x"},
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("locked account = %v, want PermissionDenied", err)
	}
}
