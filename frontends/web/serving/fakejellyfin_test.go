package serving

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeJellyfin is a self-contained Jellyfin serve mock. It answers exactly
// the REST contract the jellyfin adapter speaks (authenticate, index,
// play-back info, direct stream) — the same contract the core adapter unit
// suites and the core M5 fixtures exercise — so a browser flow test drives
// the unmodified adapter end to end. It carries no state beyond its catalog,
// copied from the provider slot the core fixtures seed.
type fakeJellyfin struct {
	mu          sync.Mutex
	srv         *httptest.Server
	items       []jfCatalogItem
	streamBytes map[string][]byte
	streamHits  map[string]int
}

// jfCatalogItem is the subset of Jellyfin's BaseItemDto the fake index holds.
type jfCatalogItem struct {
	ID        string            `json:"Id"`
	Type      string            `json:"Type"`
	Name      string            `json:"Name"`
	Year      int               `json:"ProductionYear"`
	Providers map[string]string `json:"ProviderIds"`
}

// startFakeJellyfin starts the fake and registers its cleanup.
func startFakeJellyfin(t *testing.T) *fakeJellyfin {
	t.Helper()
	f := &fakeJellyfin{
		items: []jfCatalogItem{
			{
				ID: "movie-gondwana", Type: "Movie", Name: "The Last Gondwana Gardener", Year: 2021,
				Providers: map[string]string{"Imdb": "tt-gondwana", "Tmdb": "12"},
			},
			{
				ID: "movie-coral", Type: "Movie", Name: "Coral Skies", Year: 2019,
				Providers: map[string]string{"Tmdb": "23"},
			},
			{
				ID: "series-tidal", Type: "Series", Name: "Tidal Station", Year: 2022,
				Providers: map[string]string{"Tvdb": "99"},
			},
		},
		streamBytes: map[string][]byte{
			"movie-gondwana": []byte("P5-gondwana-mkv-bytes"),
			"movie-coral":    []byte("P5-coral-mkv-bytes"),
			"series-tidal":   []byte("P5-tidal-mkv-bytes"),
		},
		streamHits: map[string]int{},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.ServeHTTP))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeJellyfin) URL() string { return f.srv.URL }

func (f *fakeJellyfin) Hits(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.streamHits[id]
}

func (f *fakeJellyfin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/Users/AuthenticateByName":
		f.authenticate(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/Items":
		f.index(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/Items/") && strings.HasSuffix(r.URL.Path, "/PlaybackInfo"):
		f.playbackInfo(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/Videos/") && strings.HasSuffix(r.URL.Path, "/stream"):
		f.stream(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeJellyfin) authenticate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"Username"`
		Pw       string `json:"Pw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"AccessToken":"srv-access-token","User":{"Id":"jf-user-home"}}`))
}

func (f *fakeJellyfin) index(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "MediaBrowser Token=") {
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	wantKinds := map[string]bool{}
	for _, k := range strings.Split(q.Get("includeItemTypes"), ",") {
		wantKinds[k] = true
	}
	all := make([]jfCatalogItem, 0, len(f.items))
	for _, it := range f.items {
		if len(wantKinds) == 0 || wantKinds[it.Type] {
			all = append(all, it)
		}
	}
	start, _ := strconv.Atoi(q.Get("startIndex"))
	if limit, err := strconv.Atoi(q.Get("limit")); err == nil && limit > 0 && start+limit < len(all) {
		all = all[start : start+limit]
	} else if start < len(all) {
		all = all[start:]
	} else {
		all = nil
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Items            []jfCatalogItem `json:"Items"`
		TotalRecordCount int             `json:"TotalRecordCount"`
		StartIndex       int             `json:"StartIndex"`
	}{Items: all, TotalRecordCount: len(f.items), StartIndex: start})
}

func (f *fakeJellyfin) playbackInfo(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "MediaBrowser Token=") {
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/Items/"), "/PlaybackInfo")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		MediaSources []map[string]any `json:"MediaSources"`
	}{
		MediaSources: []map[string]any{{
			"Id":        "ms-" + id,
			"Container": "mkv",
			"MediaStreams": []map[string]any{
				{"Type": "Video", "Codec": "hevc", "Width": 1920, "Height": 1080, "Language": "eng"},
				{"Type": "Audio", "Codec": "truehd", "Channels": 8, "ChannelLayout": "7.1", "Language": "eng"},
				{"Type": "Subtitle", "Codec": "srt", "Language": "eng"},
			},
		}},
	})
}

func (f *fakeJellyfin) stream(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("api_key") != "srv-access-token" {
		http.Error(w, "missing api key", http.StatusUnauthorized)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/Videos/"), "/stream")
	f.mu.Lock()
	f.streamHits[id]++
	body := append([]byte(nil), f.streamBytes[id]...)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}
