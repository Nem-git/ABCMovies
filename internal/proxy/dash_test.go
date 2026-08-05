package proxy_test

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/internal/proxy"
	"github.com/nem-git/abcmovies/internal/stream"
)

// --- DASH test fixtures: real-world-style MPD manifests ---

// Fixture 1: SegmentTemplate at AdaptationSet level, $Number$ mode
const testDASHNumberMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-live:2011" type="static" minBufferTime="PT2S">
  <Period id="p0" start="PT0S">
    <AdaptationSet mimeType="video/mp4" segmentAlignment="true">
      <SegmentTemplate timescale="90000" media="$RepresentationID$/seg_$Number$.m4s" initialization="$RepresentationID$/init.mp4" startNumber="1"/>
      <Representation id="v1" bandwidth="5000000" width="1920" height="1080"/>
      <Representation id="v2" bandwidth="2500000" width="1280" height="720"/>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4" segmentAlignment="true">
      <SegmentTemplate timescale="44100" media="$RepresentationID$/seg_$Number$.m4s" initialization="$RepresentationID$/init.mp4" startNumber="1"/>
      <Representation id="a1" bandwidth="128000"/>
    </AdaptationSet>
  </Period>
</MPD>`

// Fixture 2: SegmentTemplate with $Time$ and SegmentTimeline
const testDASHTimeMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-live:2011" type="static" minBufferTime="PT2S">
  <Period id="p0">
    <AdaptationSet mimeType="video/mp4">
      <SegmentTemplate timescale="90000" media="$RepresentationID$/$Time$.m4s" initialization="$RepresentationID$/init.mp4">
        <SegmentTimeline>
          <S t="0" d="270000" r="2"/>
        </SegmentTimeline>
      </SegmentTemplate>
      <Representation id="v1" bandwidth="3000000"/>
    </AdaptationSet>
  </Period>
</MPD>`

// Fixture 3: SegmentTemplate at Period level (inherited)
const testDASHPeriodInheritMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-live:2011" type="static" minBufferTime="PT2S">
  <Period id="p0">
    <SegmentTemplate timescale="90000" media="chunks/$RepresentationID$_$Number$.m4s" initialization="$RepresentationID$_init.mp4" startNumber="0"/>
    <AdaptationSet mimeType="video/mp4">
      <Representation id="v1" bandwidth="5000000"/>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4">
      <Representation id="a1" bandwidth="128000"/>
    </AdaptationSet>
  </Period>
</MPD>`

// Fixture 4: $Number$ with format suffix
const testDASHNumberFormatMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-live:2011" type="static" minBufferTime="PT2S">
  <Period id="p0">
    <AdaptationSet mimeType="video/mp4">
      <SegmentTemplate timescale="90000" media="$RepresentationID$/seg_$Number%05d$.m4s" initialization="$RepresentationID$/init.mp4" startNumber="0"/>
      <Representation id="v1" bandwidth="5000000"/>
    </AdaptationSet>
  </Period>
</MPD>`

// Fixture 5: SegmentBase representation
const testDASHSegmentBaseMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011" type="static" minBufferTime="PT2S">
  <Period id="p0">
    <AdaptationSet mimeType="video/mp4">
      <Representation id="v1" bandwidth="4190760" width="1920" height="1080">
        <BaseURL>car_cenc.mp4</BaseURL>
        <SegmentBase indexRange="2755-3230">
          <Initialization range="0-2754"/>
        </SegmentBase>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

// Fixture 6: $Bandwidth$ used in media template
const testDASHBandwidthMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-live:2011" type="static" minBufferTime="PT2S">
  <Period id="p0">
    <AdaptationSet mimeType="video/mp4">
      <SegmentTemplate timescale="90000" media="$RepresentationID$_$Bandwidth$_$Number$.m4s" initialization="$RepresentationID$_init.mp4" startNumber="0"/>
      <Representation id="v1" bandwidth="5000000"/>
    </AdaptationSet>
  </Period>
</MPD>`

func newDASHStrategy(fetch proxy.Fetcher, store proxy.StateStore) *proxy.DASHStrategy {
	return proxy.NewDASHStrategy(proxy.StrategyDeps{Fetcher: fetch, State: store})
}

func dashTestMeta(proxyBase string) *proxy.StreamMeta {
	return &proxy.StreamMeta{
		ProviderTag:  "test",
		ContentKey:   "movies:1",
		StreamFile:   "manifest.mpd",
		Format:       "dash",
		ProxyBaseURL: proxyBase,
		Headers:      http.Header{"X-Auth": []string{"token123"}},
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}
}

// --- Tests ---

func TestDASHStrategy_SegmentTemplateRewrite_Number(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/dash"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testDASHNumberMPD), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	var buf strings.Builder
	meta := dashTestMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/manifest.mpd"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()

	// Media template rewritten with period prefix and $RepresentationID$ resolved
	if !strings.Contains(result, "periods/0/adaptation-sets/0/representations/v1/segments/seg_$Number$.m4s") {
		t.Errorf("v1 media template not rewritten correctly, got:\n%s", result)
	}

	// Init template rewritten with $RepresentationID$ resolved, $Number$ stripped
	if !strings.Contains(result, "periods/0/adaptation-sets/0/representations/v1/segments/init") {
		t.Errorf("v1 init template not rewritten correctly, got:\n%s", result)
	}

	// State stored for v1, v2, a1
	ctx := t.Context()
	for _, tc := range []struct {
		repID, bandwidth string
		period, as       int
	}{
		{"v1", "5000000", 0, 0},
		{"v2", "2500000", 0, 0},
		{"a1", "128000", 0, 1},
	} {
		key := proxy.DASHStateKey("test", "movies:1", tc.period, tc.as, tc.repID)
		got, found, err := store.Get(ctx, key)
		if err != nil {
			t.Fatalf("state.Get(%q) error: %v", key, err)
		}
		if !found {
			t.Errorf("state not found for rep %s", tc.repID)
			continue
		}
		if got.UpstreamRepID != tc.repID {
			t.Errorf("rep %s: UpstreamRepID = %q, want %q", tc.repID, got.UpstreamRepID, tc.repID)
		}
		if got.UpstreamBandwidth != tc.bandwidth {
			t.Errorf("rep %s: UpstreamBandwidth = %q, want %q", tc.repID, got.UpstreamBandwidth, tc.bandwidth)
		}
		// Original templates stored (with $RepresentationID$ and $Bandwidth$ intact)
		if !strings.Contains(got.UpstreamMediaTemplate, "$RepresentationID$") {
			t.Errorf("rep %s: original media template should contain $RepresentationID$, got %q", tc.repID, got.UpstreamMediaTemplate)
		}
		if !strings.Contains(got.UpstreamMediaTemplate, "$Number$") {
			t.Errorf("rep %s: original media template should contain $Number$, got %q", tc.repID, got.UpstreamMediaTemplate)
		}
	}
}

func TestDASHStrategy_SegmentTemplateRewrite_Time(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/dash"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testDASHTimeMPD), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	var buf strings.Builder
	meta := dashTestMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/manifest.mpd"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()

	// Media template has $Time$ as placeholder
	if !strings.Contains(result, "periods/0/adaptation-sets/0/representations/v1/segments/$Time$.m4s") {
		t.Errorf("v1 media template missing $Time$ placeholder, got:\n%s", result)
	}
	// $RepresentationID$ resolved in media template
	if strings.Contains(result, "$RepresentationID$") {
		t.Errorf("$RepresentationID$ should be resolved in media template, got:\n%s", result)
	}
}

func TestDASHStrategy_PeriodLevelInheritance(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/dash"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testDASHPeriodInheritMPD), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	var buf strings.Builder
	meta := dashTestMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/manifest.mpd"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()

	// Video representation (v1 in AS 0)
	if !strings.Contains(result, "periods/0/adaptation-sets/0/representations/v1/segments/v1_$Number$.m4s") {
		t.Errorf("v1 media template not rewritten correctly from Period inheritance, got:\n%s", result)
	}
	// Audio representation (a1 in AS 1)
	if !strings.Contains(result, "periods/0/adaptation-sets/1/representations/a1/segments/a1_$Number$.m4s") {
		t.Errorf("a1 media template not rewritten correctly from Period inheritance, got:\n%s", result)
	}
	// Init templates
	if !strings.Contains(result, "periods/0/adaptation-sets/0/representations/v1/segments/init") {
		t.Errorf("v1 init template not rewritten, got:\n%s", result)
	}
	if !strings.Contains(result, "periods/0/adaptation-sets/1/representations/a1/segments/init") {
		t.Errorf("a1 init template not rewritten, got:\n%s", result)
	}
}

func TestDASHStrategy_NumberFormatSuffix(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/dash"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testDASHNumberFormatMPD), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	var buf strings.Builder
	meta := dashTestMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/manifest.mpd"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()

	// Media template preserves format suffix
	if !strings.Contains(result, "periods/0/adaptation-sets/0/representations/v1/segments/seg_$Number%05d$.m4s") {
		t.Errorf("media template format suffix not preserved, got:\n%s", result)
	}
	// Init template stripped of filename
	if !strings.Contains(result, "periods/0/adaptation-sets/0/representations/v1/segments/init") {
		t.Errorf("init template should use the fixed /segments/init path, got:\n%s", result)
	}

	// Verify original template stored with suffix intact
	ctx := t.Context()
	key := proxy.DASHStateKey("test", "movies:1", 0, 0, "v1")
	got, found, err := store.Get(ctx, key)
	if err != nil || !found {
		t.Fatalf("state not found for v1")
	}
	if !strings.Contains(got.UpstreamMediaTemplate, "$Number%05d$") {
		t.Errorf("stored media template should preserve format suffix, got %q", got.UpstreamMediaTemplate)
	}
}

func TestDASHStrategy_MultipleRepresentations(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/dash"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testDASHNumberMPD), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	var buf strings.Builder
	meta := dashTestMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/manifest.mpd"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	ctx := t.Context()

	// Three representations: v1, v2 (video AS 0), a1 (audio AS 1)
	for _, tc := range []struct {
		repID, bandwidth, period, as string
	}{
		{"v1", "5000000", "0", "0"},
		{"v2", "2500000", "0", "0"},
		{"a1", "128000", "0", "1"},
	} {
		periodIdx, _ := strconv.Atoi(tc.period)
		asIdx, _ := strconv.Atoi(tc.as)
		key := proxy.DASHStateKey("test", "movies:1", periodIdx, asIdx, tc.repID)
		_, found, err := store.Get(ctx, key)
		if err != nil {
			t.Fatalf("state.Get(%q) error: %v", key, err)
		}
		if !found {
			t.Errorf("state not found for rep %s (period=%s, as=%s)", tc.repID, tc.period, tc.as)
		}
	}
}

func TestDASHStrategy_SegmentServing(t *testing.T) {
	var fetchedURL string
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		fetchedURL = url
		return bytesReader("segment-data"), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	// The strategy's ServeSegment is a passthrough: it fetches locator.URL.
	// URL reconstruction from state happens at the Proxy level (Proxy.ServeDASHSegment).
	// So we reconstruct the URL as the proxy would and pass it in the locator.
	ctx := t.Context()
	key := proxy.DASHStateKey("test", "movies:1", 0, 0, "v1")
	store.Put(ctx, key, proxy.StreamMeta{
		ProviderTag:           "test",
		ContentKey:            "movies:1",
		UpstreamMediaTemplate: "https://cdn.example.com/movie/$RepresentationID$/seg_$Number$.m4s",
		UpstreamRepID:         "v1",
		UpstreamBandwidth:     "5000000",
		Headers:               http.Header{"X-Auth": []string{"token123"}},
		ExpiresAt:             time.Now().Add(5 * time.Minute),
	})

	// Reconstruct URL as Proxy.ServeDASHSegment would: resolve $RepresentationID$, $Bandwidth$, $Number$
	upstreamURL := "https://cdn.example.com/movie/v1/seg_7.m4s"

	var buf strings.Builder
	err := s.ServeSegment(t.Context(), &buf, stream.Locator{URL: upstreamURL, Headers: http.Header{"X-Auth": []string{"token123"}}}, "7")
	if err != nil {
		t.Fatalf("ServeSegment() error: %v", err)
	}

	want := "https://cdn.example.com/movie/v1/seg_7.m4s"
	if fetchedURL != want {
		t.Errorf("upstream fetched URL = %q, want %q", fetchedURL, want)
	}
}

func TestDASHStrategy_SegmentServing_TimeMode(t *testing.T) {
	var fetchedURL string
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		fetchedURL = url
		return bytesReader("segment-data"), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	ctx := t.Context()
	key := proxy.DASHStateKey("test", "movies:1", 0, 0, "v1")
	store.Put(ctx, key, proxy.StreamMeta{
		ProviderTag:           "test",
		ContentKey:            "movies:1",
		UpstreamMediaTemplate: "https://cdn.example.com/movie/$RepresentationID$/$Time$.m4s",
		UpstreamRepID:         "v1",
		UpstreamBandwidth:     "3000000",
		Headers:               http.Header{},
		ExpiresAt:             time.Now().Add(5 * time.Minute),
	})

	// Strategy's ServeSegment is a passthrough — reconstruct URL as the proxy would
	upstreamURL := "https://cdn.example.com/movie/v1/270000.m4s"

	var buf strings.Builder
	err := s.ServeSegment(t.Context(), &buf, stream.Locator{URL: upstreamURL}, "270000")
	if err != nil {
		t.Fatalf("ServeSegment() error: %v", err)
	}

	want := "https://cdn.example.com/movie/v1/270000.m4s"
	if fetchedURL != want {
		t.Errorf("upstream fetched URL = %q, want %q", fetchedURL, want)
	}
}

func TestDASHStrategy_InitServing(t *testing.T) {
	var fetchedURL string
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		fetchedURL = url
		return bytesReader("init-data"), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	ctx := t.Context()
	key := proxy.DASHStateKey("test", "movies:1", 0, 0, "v1")
	store.Put(ctx, key, proxy.StreamMeta{
		ProviderTag:          "test",
		ContentKey:           "movies:1",
		UpstreamInitTemplate: "https://cdn.example.com/movie/$RepresentationID$/init.mp4",
		UpstreamRepID:        "v1",
		UpstreamBandwidth:    "5000000",
		Headers:              http.Header{"X-Auth": []string{"token123"}},
		ExpiresAt:            time.Now().Add(5 * time.Minute),
	})

	// Strategy's ServeInitSegment is a passthrough — reconstruct URL as the proxy would
	upstreamURL := "https://cdn.example.com/movie/v1/init.mp4"

	var buf strings.Builder
	err := s.ServeInitSegment(t.Context(), &buf, stream.Locator{URL: upstreamURL, Headers: http.Header{"X-Auth": []string{"token123"}}})
	if err != nil {
		t.Fatalf("ServeInitSegment() error: %v", err)
	}

	want := "https://cdn.example.com/movie/v1/init.mp4"
	if fetchedURL != want {
		t.Errorf("upstream fetched URL = %q, want %q", fetchedURL, want)
	}
}

func TestDASHStrategy_StateKeys(t *testing.T) {
	tests := []struct {
		tag, contentKey string
		period, as      int
		repID           string
		want            string
	}{
		{"T", "movies:1", 0, 0, "v1", "T:movies:1:dash:0:0:v1"},
		{"T", "movies:1", 0, 1, "a1", "T:movies:1:dash:0:1:a1"},
		{"S", "series:456:789:101", 1, 2, "v3", "S:series:456:789:101:dash:1:2:v3"},
	}
	for _, tt := range tests {
		got := proxy.DASHStateKey(tt.tag, tt.contentKey, tt.period, tt.as, tt.repID)
		if got != tt.want {
			t.Errorf("DASHStateKey(%q, %q, %d, %d, %q) = %q, want %q", tt.tag, tt.contentKey, tt.period, tt.as, tt.repID, got, tt.want)
		}
	}
}

func TestDASHStrategy_SegmentBase(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/dash"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testDASHSegmentBaseMPD), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	var buf strings.Builder
	meta := dashTestMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/manifest.mpd"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()

	// BaseURL should be resolved to absolute URL
	if !strings.Contains(result, "car_cenc.mp4") {
		t.Errorf("BaseURL not preserved in output, got:\n%s", result)
	}
	// No SegmentTemplate rewriting (SegmentBase path)
	// Just verify the output is valid XML and contains the representation
	if !strings.Contains(result, `id="v1"`) {
		t.Errorf("representation v1 not found in output, got:\n%s", result)
	}
}

func TestDASHStrategy_BandwidthInMediaTemplate(t *testing.T) {
	proxyBase := "/services/test/movies/1/streams/dash"
	fetch := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		return bytesReader(testDASHBandwidthMPD), nil, nil
	})
	store := proxy.NewMemoryStore(time.Minute)
	s := newDASHStrategy(fetch, store)

	var buf strings.Builder
	meta := dashTestMeta(proxyBase)
	locator := stream.Locator{URL: "https://cdn.example.com/movie/manifest.mpd"}

	_, err := s.ServeManifest(t.Context(), &buf, locator, meta)
	if err != nil {
		t.Fatalf("ServeManifest() error: %v", err)
	}

	result := buf.String()

	// $Bandwidth$ should be resolved to the representation's literal value
	if !strings.Contains(result, "periods/0/adaptation-sets/0/representations/v1/segments/v1_5000000_$Number$.m4s") {
		t.Errorf("$Bandwidth$ should be resolved in media template, got:\n%s", result)
	}
	// $RepresentationID$ resolved in media template
	if strings.Contains(result, "$RepresentationID$") {
		t.Errorf("$RepresentationID$ should be resolved in media template, got:\n%s", result)
	}

	// Verify original template stored with $Bandwidth$ intact
	ctx := t.Context()
	key := proxy.DASHStateKey("test", "movies:1", 0, 0, "v1")
	got, found, err := store.Get(ctx, key)
	if err != nil || !found {
		t.Fatalf("state not found for v1")
	}
	if !strings.Contains(got.UpstreamMediaTemplate, "$Bandwidth$") {
		t.Errorf("stored media template should contain $Bandwidth$, got %q", got.UpstreamMediaTemplate)
	}
	if !strings.Contains(got.UpstreamMediaTemplate, "$RepresentationID$") {
		t.Errorf("stored media template should contain $RepresentationID$, got %q", got.UpstreamMediaTemplate)
	}

	// Segment serving: reconstruct URL as the proxy would
	var fetchedURL string
	fetch2 := newFetcherMock(func(url string) (io.ReadCloser, http.Header, error) {
		fetchedURL = url
		return bytesReader("data"), nil, nil
	})
	s2 := newDASHStrategy(fetch2, store)

	// Proxy.ServeDASHSegment reconstructs: resolve $RepresentationID$, $Bandwidth$, $Number$
	upstreamURL := "https://cdn.example.com/movie/v1_5000000_3.m4s"

	var segBuf strings.Builder
	err = s2.ServeSegment(t.Context(), &segBuf, stream.Locator{URL: upstreamURL}, "3")
	if err != nil {
		t.Fatalf("ServeSegment() error: %v", err)
	}

	want := "https://cdn.example.com/movie/v1_5000000_3.m4s"
	if fetchedURL != want {
		t.Errorf("upstream fetched URL = %q, want %q", fetchedURL, want)
	}
}
