package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

// fakeJellyfin is a minimal in-process provider: it answers one login and one
// catalogue page, so a Stack's jellyfin slot builds against it
// (PLAN.md §3.3 handshake over-config is driven by the real adapter). It also
// serves the delivery path (PlaybackInfo + stream) so a composed stack can
// start a play against it.
type fakeJellyfin struct {
	srv    *httptest.Server
	authN  int
	itemsN int
	// playback401 makes every PlaybackInfo call fail authorization: a test
	// flips it to kill a linked account's cached session at the provider.
	// The adapter blanks its vaulted copy on rejection, so the next resolve
	// discovers a dead session — the re-auth flow's trigger (PLAN.md §7.5).
	playback401 bool
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
	mux.HandleFunc("POST /Items/{id}/PlaybackInfo", func(w http.ResponseWriter, r *http.Request) {
		if f.playback401 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"Exception":{"Message":"Unauthorized"}}`))
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "MediaBrowser Token=") {
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/Items/"), "/PlaybackInfo")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"MediaSources": []map[string]any{{
				"Id":        "ms-" + id,
				"Container": "mkv",
				"MediaStreams": []map[string]any{
					{"Type": "Video", "Codec": "hevc", "Width": 1920, "Height": 1080, "Language": "eng"},
					{"Type": "Audio", "Codec": "truehd", "Channels": 8, "ChannelLayout": "7.1", "Language": "eng"},
					{"Type": "Subtitle", "Codec": "srt", "Language": "eng"},
				},
			}},
		})
	})
	mux.HandleFunc("GET /Videos/{id}/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "faker-token" {
			http.Error(w, "missing api key", http.StatusUnauthorized)
			return
		}
		body := []byte("fake-mkv-bytes")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
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
stores:
  slots:
    backend: in-memory
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("JF_TEST_PASSWORD", "operator-password-not-real")

	stack, err := New(context.Background(), configPath, slog.Default())
	if err != nil {
		t.Fatalf("stack: %v", err)
	}
	seedJellyfinSlots(t, stack, "jf", fake.URL())
	srv, ok := stack.Service().(*apiserver.Server)
	if !ok {
		t.Fatalf("service is not a concrete *apiserver.Server")
	}

	ctx := apiserver.WithUserID(context.Background(), "u1")
	// LinkAccountRequest.Visibility has proto3 explicit presence (optional).
	vis := apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE
	linkRes, err := srv.LinkAccount(ctx, &apiv1.LinkAccountRequest{
		Provider: "jellyfin",
		BaseUrl:  fake.URL(),
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{Username: "u1link", Password: []byte("pw")},
		},
		Visibility: &vis,
	})
	if err != nil {
		t.Fatalf("LinkAccount: %v", err)
	}

	// The linked account must be live for its owner immediately: reachable
	// through the library's reach set, no restart, no reload.
	reach, ok := stack.Slots().Library.ReachAuthorized(linkRes.GetAccountId(), "u1")
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
