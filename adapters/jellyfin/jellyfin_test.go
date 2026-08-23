package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// fakeJellyfin is a minimal in-process Jellyfin: it authenticates one user and
// serves a fixed item index with offset pagination, counting requests.
type fakeJellyfin struct {
	server     *httptest.Server
	items      []mediaItem
	logins     atomic.Int64
	itemCalls  atomic.Int64
	unauthOnce atomic.Bool // serve exactly one 401 on /Items to exercise re-auth
}

func newFake(t *testing.T, items []mediaItem) *fakeJellyfin {
	t.Helper()
	f := &fakeJellyfin{items: items}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /Users/AuthenticateByName", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string
			Pw       string
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username != "bob" || req.Pw != "sekret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.logins.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"AccessToken": "test-access-token",
			"User":        map[string]any{"Id": "user-1"},
		})
	})
	mux.HandleFunc("GET /Items", func(w http.ResponseWriter, r *http.Request) {
		f.itemCalls.Add(1)
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("fields") != "ProviderIds" {
			// Jellyfin 10.9+ omits ProviderIds unless explicitly requested;
			// a request without it would silently lose identity assertions.
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if f.unauthOnce.CompareAndSwap(true, false) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		offset := queryInt(r, "startIndex")
		limit := queryInt(r, "limit")
		end := min(offset+limit, len(f.items))
		page := []mediaItem{}
		if offset < len(f.items) {
			page = f.items[offset:end]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Items":            page,
			"TotalRecordCount": len(f.items),
			"StartIndex":       offset,
		})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func queryInt(r *http.Request, key string) int {
	var v int
	_, _ = fmt.Sscanf(r.URL.Query().Get(key), "%d", &v)
	return v
}

const testPasswordEnv = "JELLYFIN_TEST_PASSWORD"

func newTestSlot(t *testing.T, f *fakeJellyfin) *Slot {
	t.Helper()
	t.Setenv(testPasswordEnv, "sekret")
	slot, err := New([]Account{{
		ID:          "primary",
		URL:         f.server.URL,
		Username:    "bob",
		PasswordEnv: testPasswordEnv,
	}}, WithHTTPClient(f.server.Client()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return slot
}

func TestCapabilityQueryDeclaresBrowseV1(t *testing.T) {
	f := newFake(t, nil)
	slot := newTestSlot(t, f)
	resp, err := slot.CapabilityQuery(context.Background(), nil)
	if err != nil {
		t.Fatalf("CapabilityQuery: %v", err)
	}
	got := map[string]uint32{}
	for _, c := range resp.GetCapabilities() {
		got[c.GetName()] = c.GetVersion()
	}
	if got["meta"] != 1 || got["browse"] != 1 {
		t.Fatalf("capabilities = %v, want meta v1 + browse v1", got)
	}
}

func TestAuthenticateFailsFastOnBadCredentials(t *testing.T) {
	f := newFake(t, nil)
	slot := newTestSlot(t, f)
	t.Setenv(testPasswordEnv, "wrong-password")
	// ensureSession caches nothing on failure; Authenticate surfaces the error.
	if err := slot.Authenticate(context.Background()); err == nil {
		t.Fatal("Authenticate succeeded with wrong credentials")
	}
}

func TestCatalogueSyncPaginatesWholeCatalogue(t *testing.T) {
	items := make([]mediaItem, 0, pageSize+7)
	for i := range pageSize + 7 {
		items = append(items, mediaItem{
			Id:   fmt.Sprintf("item-%04d", i),
			Type: "Movie",
			Name: fmt.Sprintf("Film %d", i),
		})
	}
	f := newFake(t, items)
	slot := newTestSlot(t, f)
	ctx := context.Background()

	req := &slotsv1.CatalogueSyncRequest{AccountId: "primary"}
	total := 0
	for page := 0; ; page++ {
		if page > 10 {
			t.Fatal("pagination did not terminate")
		}
		resp, err := slot.CatalogueSync(ctx, req)
		if err != nil {
			t.Fatalf("CatalogueSync page %d: %v", page, err)
		}
		total += len(resp.GetItems())
		if resp.GetNextPageToken() == "" {
			break
		}
		req.PageToken = resp.GetNextPageToken()
	}
	if total != pageSize+7 {
		t.Fatalf("synced %d items, want %d", total, pageSize+7)
	}
}

func TestCatalogueSyncMapsKindsYearsAndExternalIds(t *testing.T) {
	f := newFake(t, []mediaItem{
		{
			Id: "m1", Type: "Movie", Name: "The Shawshank Redemption", ProductionYear: 1994,
			ProviderIds: map[string]string{"Imdb": "tt0111161", "Tmdb": "278"},
		},
		{Id: "s1", Type: "Series", Name: "Breaking Bad"},
	})
	slot := newTestSlot(t, f)
	resp, err := slot.CatalogueSync(context.Background(), &slotsv1.CatalogueSyncRequest{AccountId: "primary"})
	if err != nil {
		t.Fatalf("CatalogueSync: %v", err)
	}
	if len(resp.GetItems()) != 2 {
		t.Fatalf("got %d items, want 2", len(resp.GetItems()))
	}
	movie := resp.GetItems()[0]
	if movie.GetKind() != slotsv1.ItemKind_ITEM_KIND_MOVIE ||
		movie.GetNativeId() != "m1" ||
		movie.GetMetadata().GetTitle() != "The Shawshank Redemption" ||
		movie.GetMetadata().GetYear() != 1994 {
		t.Fatalf("movie mapping wrong: %+v", movie)
	}
	if len(movie.GetExternalIds()) != 2 {
		t.Fatalf("got %d external ids, want 2", len(movie.GetExternalIds()))
	}
	ns := map[string]string{}
	for _, id := range movie.GetExternalIds() {
		ns[id.GetNamespace()] = id.GetValue()
	}
	if ns["imdb"] != "tt0111161" || ns["tmdb"] != "278" {
		t.Fatalf("external ids not lower-cased/mapped: %v", ns)
	}
	series := resp.GetItems()[1]
	if series.GetKind() != slotsv1.ItemKind_ITEM_KIND_SERIES || series.GetMetadata().GetYear() != 0 {
		t.Fatalf("series mapping wrong: %+v", series)
	}
	if resp.GetNextPageToken() != "" {
		t.Fatalf("single-page catalogue must end iteration, got token %q", resp.GetNextPageToken())
	}
}

func TestCatalogueSyncEmptyLibraryIsOneEmptyFinalPage(t *testing.T) {
	f := newFake(t, nil)
	slot := newTestSlot(t, f)
	resp, err := slot.CatalogueSync(context.Background(), &slotsv1.CatalogueSyncRequest{AccountId: "primary"})
	if err != nil {
		t.Fatalf("CatalogueSync: %v", err)
	}
	if len(resp.GetItems()) != 0 || resp.GetNextPageToken() != "" {
		t.Fatalf("empty library must yield one empty final page, got %+v", resp)
	}
}

func TestCatalogueSyncRejectsMalformedTokenAndUnknownAccount(t *testing.T) {
	f := newFake(t, nil)
	slot := newTestSlot(t, f)
	ctx := context.Background()

	if _, err := slot.CatalogueSync(ctx, &slotsv1.CatalogueSyncRequest{AccountId: "primary", PageToken: "garbage"}); err == nil {
		t.Fatal("malformed page_token accepted")
	}
	if _, err := slot.CatalogueSync(ctx, &slotsv1.CatalogueSyncRequest{AccountId: "nope"}); err == nil {
		t.Fatal("unknown account accepted")
	}
}

func TestCatalogueSyncReauthenticatesOnceAfter401(t *testing.T) {
	f := newFake(t, []mediaItem{{Id: "m1", Type: "Movie", Name: "X"}})
	f.unauthOnce.Store(true)
	slot := newTestSlot(t, f)
	resp, err := slot.CatalogueSync(context.Background(), &slotsv1.CatalogueSyncRequest{AccountId: "primary"})
	if err != nil {
		t.Fatalf("CatalogueSync after 401: %v", err)
	}
	if len(resp.GetItems()) != 1 {
		t.Fatalf("expected the retry to return the page, got %+v", resp)
	}
	if f.logins.Load() != 2 {
		t.Fatalf("logins = %d, want 2 (initial + re-auth)", f.logins.Load())
	}
}

func TestNewRejectsIncompleteAccounts(t *testing.T) {
	_ = os.Unsetenv(testPasswordEnv)
	for _, a := range []Account{
		{ID: "", URL: "http://x", Username: "u", PasswordEnv: "P"},
		{ID: "a", URL: "", Username: "u", PasswordEnv: "P"},
		{ID: "a", URL: "http://x", Username: "", PasswordEnv: "P"},
		{ID: "a", URL: "http://x", Username: "u", PasswordEnv: ""},
	} {
		if _, err := New([]Account{a}); err == nil {
			t.Fatalf("incomplete account %+v accepted", a)
		}
	}
	if _, err := New(nil); err == nil {
		t.Fatal("zero accounts accepted")
	}
	dup := Account{ID: "a", URL: "http://x", Username: "u", PasswordEnv: "P"}
	if _, err := New([]Account{dup, dup}); err == nil {
		t.Fatal("duplicate account id accepted")
	}
}
