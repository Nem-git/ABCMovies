package slotwiring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func rec(id, provider, baseURL string) accounts.Record {
	return accounts.Record{ID: id, Provider: provider, BaseURL: baseURL, Username: "bob"}
}

// TestRouteLinkedAccounts pins the deterministic routing rule: a single
// enabled slot of the adapter catches all its linked accounts; several slots
// need a base-url match; a server with no configured slot is a provisioning
// seed (the account becomes its own user-owned server); ambiguity is an
// error, never a silent pick.
func TestRouteLinkedAccounts(t *testing.T) {
	t.Parallel()
	jfEntry := func(id, url string) config.SlotEntry {
		e := config.SlotEntry{Adapter: "jellyfin", ID: id, Enabled: true}
		if url != "" {
			e.Accounts = []config.AccountConfig{{ID: id + "-op", URL: url, Username: "op"}}
		}
		return e
	}

	t.Run("single enabled slot catches every linked account", func(t *testing.T) {
		bySlot, provisioned, err := RouteLinkedAccounts(
			[]config.SlotEntry{jfEntry("home", "http://jf-a")},
			[]accounts.Record{rec("lnk_1", "jellyfin", "http://jf-a"), rec("lnk_2", "jellyfin", "http://jf-b")})
		if err != nil {
			t.Fatalf("RouteLinkedAccounts: %v", err)
		}
		if len(bySlot["home"]) != 2 || len(provisioned) != 0 {
			t.Fatalf("bySlot=%v provisioned=%v, want both in home", bySlot, provisioned)
		}
	})

	t.Run("several slots route by base-url match", func(t *testing.T) {
		bySlot, _, err := RouteLinkedAccounts(
			[]config.SlotEntry{jfEntry("a", "http://jf-a"), jfEntry("b", "http://jf-b")},
			[]accounts.Record{rec("lnk_1", "jellyfin", "http://jf-b")})
		if err != nil {
			t.Fatalf("RouteLinkedAccounts: %v", err)
		}
		if len(bySlot["b"]) != 1 || len(bySlot["a"]) != 0 {
			t.Fatalf("want the account on slot b, got %v", bySlot)
		}
	})

	t.Run("no configured slot provisions a user-owned server", func(t *testing.T) {
		_, provisioned, err := RouteLinkedAccounts(nil, []accounts.Record{rec("lnk_1", "jellyfin", "http://jf-a")})
		if err != nil {
			t.Fatalf("RouteLinkedAccounts: %v", err)
		}
		if len(provisioned) != 1 {
			t.Fatalf("provisioned = %v, want the record", provisioned)
		}
	})

	t.Run("several slots with no server match is an error", func(t *testing.T) {
		_, _, err := RouteLinkedAccounts(
			[]config.SlotEntry{jfEntry("a", "http://jf-a"), jfEntry("b", "http://jf-b")},
			[]accounts.Record{rec("lnk_1", "jellyfin", "http://jf-nowhere")})
		if err == nil || !strings.Contains(err.Error(), "matches no enabled slot") {
			t.Fatalf("want server-match error, got %v", err)
		}
	})

	t.Run("several slots all matching is ambiguous", func(t *testing.T) {
		_, _, err := RouteLinkedAccounts(
			[]config.SlotEntry{jfEntry("a", "http://jf-a"), jfEntry("b", "http://jf-a")},
			[]accounts.Record{rec("lnk_1", "jellyfin", "http://jf-a")})
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("want ambiguity error, got %v", err)
		}
	})

	t.Run("a different adapter is not a candidate", func(t *testing.T) {
		_, provisioned, err := RouteLinkedAccounts(
			[]config.SlotEntry{{Adapter: "jellyfin", ID: "home", Enabled: true}},
			[]accounts.Record{rec("lnk_1", "tmdb", "http://x")})
		if err != nil {
			t.Fatalf("RouteLinkedAccounts: %v", err)
		}
		if len(provisioned) != 1 {
			t.Fatalf("provisioned = %v, want the tmdb record", provisioned)
		}
	})

	t.Run("disabled slots are not candidates", func(t *testing.T) {
		_, provisioned, err := RouteLinkedAccounts(
			[]config.SlotEntry{{Adapter: "jellyfin", ID: "home", Enabled: false}},
			[]accounts.Record{rec("lnk_1", "jellyfin", "http://jf-a")})
		if err != nil {
			t.Fatalf("RouteLinkedAccounts: %v", err)
		}
		if len(provisioned) != 1 {
			t.Fatalf("provisioned = %v, want the record (slot disabled)", provisioned)
		}
	})
}

// linkedFake is a minimal Jellyfin: it serves one movie to any caller that
// presents the vaulted session token, and records which token it saw. The
// whole point of the test is that no password login ever happens — the
// session must come from the vault.
type linkedFake struct {
	server *httptest.Server
	sawTok chan string
}

func newLinkedFake(t *testing.T, token string) *linkedFake {
	t.Helper()
	f := &linkedFake{sawTok: make(chan string, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /Items", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.Contains(auth, token) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		select {
		case f.sawTok <- token:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items": []map[string]any{{
				"Id": "movie-1", "Type": "Movie", "Name": "Linked Film", "ProductionYear": 2024,
			}},
			"TotalRecordCount": 1,
			"StartIndex":       0,
		})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

// TestSetupProvidersWiresLinkedAccount pins the linked-account boot path end
// to end: a linked account validated at link time (record + vaulted session
// in the store) becomes an account of the routed slot, restores its session
// from the vault (zero provider logins), syncs into the source cache under
// the slot's namespace, and appears as a library reach.
func TestSetupProvidersWiresLinkedAccount(t *testing.T) {
	vault := store.NewInMemory()
	linked := accounts.NewStore(vault, slog.Default())
	ctx := context.Background()

	token := "vaulted-session-token"
	fake := newLinkedFake(t, token)
	id := accounts.NewID()
	rec := accounts.Record{ID: id, Provider: "jellyfin", BaseURL: fake.server.URL, Username: "bob", OwnerUserID: "user-1"}
	if err := linked.Add(ctx, rec); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// The link API probes and vaults the validated session under the account
	// id — reproduce that here.
	if err := linked.Save(ctx, id, []byte(fmt.Sprintf(`{"AccessToken":%q,"User":{"Id":"u-1"}}`, token))); err != nil {
		t.Fatalf("vault session: %v", err)
	}

	reg := registry.NewInProcess()
	defer reg.Close()
	itemReg, err := itemregistry.New(store.NewInMemory(), "")
	if err != nil {
		t.Fatalf("item registry: %v", err)
	}

	jobs, reaches, _, err := SetupProviders([]config.SlotEntry{{
		Adapter: "jellyfin", ID: "home-jf", Enabled: true,
	}}, Deps{
		Registry:     reg,
		Accounts:     linked,
		SourceCache:  store.NewInMemory(),
		Logger:       slog.Default(),
		ItemRegistry: itemReg,
	})
	if err != nil {
		t.Fatalf("SetupProviders: %v", err)
	}

	if len(reaches) != 1 || reaches[0].AccountID != id {
		t.Fatalf("reaches = %v, want exactly the linked account %q", reachIDs(reaches), id)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want one refresh job", len(jobs))
	}

	// The vaulted session was used: the fake saw the token and no login route
	// was hit (no account could even try — there is no password env).
	select {
	case got := <-fake.sawTok:
		if got != token {
			t.Fatalf("fake saw token %q, want %q", got, token)
		}
	default:
		t.Fatal("intercepted sync never reached the fake using the vaulted session")
	}

	items, err := reaches[0].Sync.ListItems(ctx, id)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 || items[0].GetNativeId() != "movie-1" {
		t.Fatalf("source cache items = %+v, want movie-1", items)
	}
	if items[0].GetMetadata().GetTitle() != "Linked Film" {
		t.Fatalf("item metadata = %+v", items[0].GetMetadata())
	}
}

func reachIDs(reaches []library.Reach) []string {
	out := make([]string, 0, len(reaches))
	for _, r := range reaches {
		out = append(out, r.AccountID)
	}
	return out
}

// TestSetupProvidersProvisionsUserOwnedServer pins the user-owned-server path
// (PLAN.md §3.5): a link whose server has no configured slot is wired as its
// own synthetic slot under the server's derived namespace, resumes from the
// vault, syncs into that namespace, and is reachable for playback. A second
// user linking the same server would join the same namespace and therefore
// the same identity (PLAN.md §1.25) — ServerNamespace is shared per server.
func TestSetupProvidersProvisionsUserOwnedServer(t *testing.T) {
	vault := store.NewInMemory()
	linked := accounts.NewStore(vault, slog.Default())
	ctx := context.Background()

	token := "vaulted-session-token-2"
	fake := newLinkedFake(t, token)
	id := accounts.NewID()
	rec := accounts.Record{ID: id, Provider: "jellyfin", BaseURL: fake.server.URL, Username: "bob", OwnerUserID: "user-1"}
	if err := linked.Add(ctx, rec); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := linked.Save(ctx, id, []byte(fmt.Sprintf(`{"AccessToken":%q,"User":{"Id":"u-1"}}`, token))); err != nil {
		t.Fatalf("vault session: %v", err)
	}

	reg := registry.NewInProcess()
	defer reg.Close()
	itemReg, err := itemregistry.New(store.NewInMemory(), "")
	if err != nil {
		t.Fatalf("item registry: %v", err)
	}

	// No configured slot at all: the link must provision its own server.
	jobs, reaches, resolvers, err := SetupProviders(nil, Deps{
		Registry:     reg,
		Accounts:     linked,
		SourceCache:  store.NewInMemory(),
		Logger:       slog.Default(),
		ItemRegistry: itemReg,
	})
	if err != nil {
		t.Fatalf("SetupProviders: %v", err)
	}

	ns := ServerNamespace(rec)
	if len(reaches) != 1 || reaches[0].AccountID != id {
		t.Fatalf("reaches = %v, want exactly the provisioned account %q", reachIDs(reaches), id)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want one refresh job", len(jobs))
	}
	if _, ok := resolvers[ns]; !ok {
		t.Fatalf("resolvers lacks the provisioned slot %q (have %v)", ns, resolvers)
	}

	// The vaulted session was used: the fake saw the token and no login route
	// was hit.
	select {
	case got := <-fake.sawTok:
		if got != token {
			t.Fatalf("fake saw token %q, want %q", got, token)
		}
	default:
		t.Fatal("intercepted sync never reached the fake using the vaulted session")
	}

	items, err := reaches[0].Sync.ListItems(ctx, id)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 1 || items[0].GetNativeId() != "movie-1" {
		t.Fatalf("source cache items = %+v, want movie-1", items)
	}
}

// TestServerNamespaceIsDeterministic pins the namespace identity: equivalent
// base URLs (scheme, host case, trailing slash) collapse to one server, and
// the result is stable and namespaced.
func TestServerNamespaceIsDeterministic(t *testing.T) {
	a := ServerNamespace(accounts.Record{Provider: "jellyfin", BaseURL: "HTTP://My-Jellyfin:8096/"})
	b := ServerNamespace(accounts.Record{Provider: "jellyfin", BaseURL: "http://my-jellyfin:8096"})
	c := ServerNamespace(accounts.Record{Provider: "jellyfin", BaseURL: "http://my-jellyfin:8096"})
	if a != b || b != c {
		t.Fatalf("namespace unstable across equivalent base URLs: %q vs %q vs %q", a, b, c)
	}
	if !strings.HasPrefix(a, "srv_") {
		t.Fatalf("namespace %q lacks the srv_ prefix", a)
	}
	if canonicalServer("http://jf-a/") != "http://jf-a" {
		t.Fatalf("canonicalServer keeps the trailing slash: %q", canonicalServer("http://jf-a/"))
	}
}
