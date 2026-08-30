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

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

func TestProduceSourcesBuildsWholeMuxManifest(t *testing.T) {
	f := newFake(t, nil)
	slot := newTestSlot(t, f)
	ctx := context.Background()

	resp, err := slot.ProduceSources(ctx, &slotsv1.ProduceSourcesRequest{
		AccountId: "primary",
		NativeId:  "item-1",
	})
	if err != nil {
		t.Fatalf("ProduceSources: %v", err)
	}
	src := resp.GetSource()
	if src == nil {
		t.Fatal("ProduceSources returned no source")
	}
	if src.GetType() != corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC {
		t.Fatalf("type = %v, want STATIC", src.GetType())
	}
	if src.GetAddressable() != corev1.Addressable_ADDRESSABLE_WHOLE_MUX {
		t.Fatalf("addressable = %v, want WHOLE_MUX", src.GetAddressable())
	}
	if len(src.GetTracks()) == 0 {
		t.Fatal("no tracks in manifest")
	}
	if v := src.GetTracks()[0].GetVideo(); v == nil || v.GetCodec() != "hevc" || v.GetWidth() != 3840 {
		t.Fatalf("container track video = %+v, want hevc 3840p", v)
	}
	// The muxed container is the fetch unit: the video track carries the direct
	// stream URL and audio/subtitle tracks reference it (WHOLE_MUX, §6.2).
	if len(src.GetTracks()[0].GetDelivery().GetLocations()) == 0 {
		t.Fatal("container track has no direct stream location")
	}
	sawAudio, sawSub := false, false
	for _, tr := range src.GetTracks() {
		if tr.GetAudio() != nil {
			sawAudio = true
			if tr.GetDelivery().GetCarriedIn() != "container" {
				t.Fatalf("audio track carried_in = %q, want container", tr.GetDelivery().GetCarriedIn())
			}
		}
		if tr.GetSubtitle() != nil {
			sawSub = true
			if tr.GetDelivery().GetCarriedIn() != "container" {
				t.Fatalf("subtitle track carried_in = %q, want container", tr.GetDelivery().GetCarriedIn())
			}
		}
	}
	if !sawAudio || !sawSub {
		t.Fatalf("manifest missing audio (saw=%v) or subtitle (saw=%v)", sawAudio, sawSub)
	}
}

func TestProduceSourcesRequiresAccountAndItem(t *testing.T) {
	f := newFake(t, nil)
	slot := newTestSlot(t, f)
	ctx := context.Background()

	if _, err := slot.ProduceSources(ctx, &slotsv1.ProduceSourcesRequest{AccountId: "primary"}); err == nil {
		t.Fatal("ProduceSources with no native_id succeeded")
	}
	if _, err := slot.ProduceSources(ctx, &slotsv1.ProduceSourcesRequest{NativeId: "item-1"}); err == nil {
		t.Fatal("ProduceSources with no account_id succeeded")
	}
}

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
	mux.HandleFunc("POST /Items/{itemId}/PlaybackInfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"MediaSources": []map[string]any{{
				"Id":        "src-1",
				"Container": "mkv",
				"MediaStreams": []map[string]any{
					{"Type": "Video", "Codec": "hevc", "Width": 3840, "Height": 2160, "BitRate": 8000000},
					{"Type": "Audio", "Codec": "eac3", "Language": "eng", "Channels": 6, "ChannelLayout": "5.1"},
					{"Type": "Subtitle", "Codec": "srt", "Language": "eng", "IsForced": false},
				},
			}},
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
	if got["meta"] != 1 || got["browse"] != 1 || got["produce-sources"] != 1 {
		t.Fatalf("capabilities = %v, want meta v1 + browse v1 + produce-sources v1", got)
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

// memVault is a test-local SessionVault; the real vault lives in core and is
// off-limits to this package (Go internal).
type memVault struct {
	blobs map[string][]byte
}

func (m *memVault) Save(_ context.Context, accountID string, blob []byte) error {
	m.blobs[accountID] = blob
	return nil
}

func (m *memVault) Load(_ context.Context, accountID string) ([]byte, error) {
	blob, ok := m.blobs[accountID]
	if !ok {
		return nil, nil
	}
	return blob, nil
}

// TestVaultFirstLinkedAccountRestoresSession pins PLAN.md §3.5's custody
// model for linked accounts: the validated session arrives in the vault at
// link time, so an account with no password-env must be usable from the
// vaulted blob alone — no re-login against the provider at boot.
func TestVaultFirstLinkedAccountRestoresSession(t *testing.T) {
	f := newFake(t, nil)
	vault := &memVault{blobs: map[string][]byte{}}
	blob, _ := json.Marshal(authResult{
		AccessToken: "vaulted-token",
		User: struct {
			ID string `json:"Id"`
		}{ID: "u-1"},
	})
	if err := vault.Save(context.Background(), "lnk_abc", blob); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	slot, err := New([]Account{{
		ID:       "lnk_abc",
		URL:      f.server.URL,
		Username: "bob",
	}}, WithHTTPClient(f.server.Client()), WithSessionVault(vault))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := slot.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate with vaulted session: %v", err)
	}
	if got := f.logins.Load(); got != 0 {
		t.Fatalf("provider login happened (%d), want vault restore only", got)
	}
}

// TestVaultFirstAccountWithoutSessionNeedsRelink pins the typed failure a
// linked account returns when its vaulted session is gone and it has no
// password-env to re-login with: the re-auth flow keys off NoSessionError
// (PLAN.md §7.5).
func TestVaultFirstAccountWithoutSessionNeedsRelink(t *testing.T) {
	f := newFake(t, nil)
	slot, err := New([]Account{{
		ID:       "lnk_abc",
		URL:      f.server.URL,
		Username: "bob",
	}}, WithHTTPClient(f.server.Client()), WithSessionVault(&memVault{blobs: map[string][]byte{}}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = slot.Authenticate(context.Background())
	if _, ok := err.(NoSessionError); !ok {
		t.Fatalf("Authenticate error = %v, want NoSessionError", err)
	}
}

// TestNewAllowsVaultFirstAccount pins that an account without a password-env
// is a valid declaration (it is a linked, vault-first account), as long as it
// still names its server and username.
func TestNewAllowsVaultFirstAccount(t *testing.T) {
	f := newFake(t, nil)
	slot, err := New([]Account{{
		ID:       "lnk_abc",
		URL:      f.server.URL,
		Username: "bob",
	}}, WithHTTPClient(f.server.Client()))
	if err != nil {
		t.Fatalf("New rejected a vault-first account: %v", err)
	}
	if slot == nil {
		t.Fatal("New returned a nil slot")
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
	// An empty PasswordEnv is NOT incomplete: it declares a vault-first
	// (linked) account whose session must already be in the vault (§3.5), so
	// only id/url/username are load-bearing here.
	for _, a := range []Account{
		{ID: "", URL: "http://x", Username: "u", PasswordEnv: "P"},
		{ID: "a", URL: "", Username: "u", PasswordEnv: "P"},
		{ID: "a", URL: "http://x", Username: "", PasswordEnv: "P"},
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
