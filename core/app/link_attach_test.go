package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

// fakeJellyfin is a minimal in-process provider: it answers one login and one
// catalogue page, so a Stack's jellyfin slot builds against it
// (PLAN.md §3.3 handshake over-config is driven by the real adapter).
type fakeJellyfin struct {
	srv    *httptest.Server
	authN  int
	itemsN int
}

func newFakeJellyfin(t *testing.T) *fakeJellyfin {
	f := &fakeJellyfin{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /Users/AuthenticateByName", func(w http.ResponseWriter, r *http.Request) {
		f.authN++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"AccessToken":"faker-token","User":{"Id":"u1"}}`))
	})
	mux.HandleFunc("GET /Items", func(w http.ResponseWriter, r *http.Request) {
		f.itemsN++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{{
				"Id": "movie-1", "Type": "Movie", "Name": "Faker", "ProductionYear": 2024,
			}},
			"TotalRecordCount": 1,
			"StartIndex":       0,
		})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeJellyfin) URL() string { return f.srv.URL }

// TestLinkedAccountIsLiveWithoutRestart proves the phase-4 seam end to end:
// linking an account through the real CoreService not only stores the record
// and session, it publishes a reach for the running library — the account
// appears without any restart or reload.
func TestLinkedAccountIsLiveWithoutRestart(t *testing.T) {
	fake := newFakeJellyfin(t)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
core:
  api:
    bind: "127.0.0.1:0"
slots:
  providers:
    - adapter: jellyfin
      id: jf
      enabled: true
      server: "`+fake.URL()+`"
      accounts:
        - id: op
          username: "alice.homeserver.user"
          password-env: JF_TEST_PASSWORD
  sinks:
    - adapter: device
      id: device
      enabled: true
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("JF_TEST_PASSWORD", "operator-password-not-real")

	stack, err := Build(configPath, slog.Default())
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	slots, err := stack.BuildSlots(context.Background(), slog.Default())
	if err != nil {
		t.Fatalf("BuildSlots: %v", err)
	}
	srv, ok := stack.Service().(*apiserver.Server)
	if !ok {
		t.Fatalf("service is not a concrete *apiserver.Server")
	}

	ctx := apiserver.WithUserID(context.Background(), "u1")
	linkRes, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin",
		BaseUrl:  fake.URL(),
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "u1link", Password: []byte("pw")},
		},
		Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE,
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}

	// The linked account must be live for its owner immediately: reachable
	// through the library's reach set, no restart, no reload.
	reach, ok := slots.Library.ReachAuthorized(linkRes.GetAccountId(), "u1")
	if !ok {
		t.Fatalf("linked account %q has no live reach", linkRes.GetAccountId())
	}
	if reach.AccountID != linkRes.GetAccountId() {
		t.Fatalf("reach account %q, want %q", reach.AccountID, linkRes.GetAccountId())
	}
	if reach.Owner != "u1" {
		t.Fatalf("reach owner %q, want u1", reach.Owner)
	}
}
