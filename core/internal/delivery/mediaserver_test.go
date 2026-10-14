package delivery

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// plainSource is a non-DRM, per-track manifest (video + audio) served by a
// test server — the shape a media-server sink stages.
func plainSource(url string) *corev1.MediaSource {
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_PER_TRACK,
		Tracks: []*corev1.Track{
			{
				Id: "v1",
				Media: &corev1.Track_Video{Video: &corev1.VideoTrack{
					Codec: "h264", Bitrate: 5800000, Width: 1920, Height: 1080,
				}},
				Delivery: &corev1.TrackDelivery{Locations: []string{url + "/v1"}},
			},
			{
				Id: "a1",
				Media: &corev1.Track_Audio{Audio: &corev1.AudioTrack{
					Codec: "aac", Bitrate: 128000, Language: "en", ChannelLayout: "5.1",
				}},
				Delivery: &corev1.TrackDelivery{Locations: []string{url + "/a1"}},
			},
		},
	}
}

func plainServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("track-bytes"))
	}))
}

func TestMediaServerSinkRequiresRelayAndOpener(t *testing.T) {
	t.Parallel()
	if _, err := (&MediaServerSinkFactory{}).NewSink(context.Background(), &Session{ID: "s"}, nil); err == nil {
		t.Fatal("expected error without relay")
	}
	if _, err := (&MediaServerSinkFactory{Relay: NewRelay()}).NewSink(context.Background(), &Session{ID: "s"}, nil); err == nil {
		t.Fatal("expected error without opener")
	}
}

func TestMediaServerSinkIsPull(t *testing.T) {
	t.Parallel()
	if (&MediaServerSink{}).Pushes() {
		t.Fatal("media-server sink must be a pull sink")
	}
}

func TestMediaServerSinkStagesManifestAndServesBytes(t *testing.T) {
	srv := plainServer(t)
	defer srv.Close()
	relay := NewRelay()
	opener := NewOpener(nil, nil, nil)

	factory := &MediaServerSinkFactory{Relay: relay, Opener: opener, TTL: time.Minute}
	sess := &Session{ID: "del-1", Context: corev1.DeliveryContext{Provider: "p", NativeId: "n"}}
	tracks := plainSource(srv.URL).Tracks
	sink, err := factory.NewSink(context.Background(), sess, tracks)
	if err != nil {
		t.Fatalf("NewSink: %v", err)
	}
	ms := sink.(*MediaServerSink)

	for _, tr := range tracks {
		if _, err := sink.Deliver(context.Background(), sess, tr, nil); err != nil {
			t.Fatalf("Deliver: %v", err)
		}
	}
	v1Tok, ok := ms.TrackToken("v1")
	if !ok {
		t.Fatal("v1 not staged")
	}
	a1Tok, ok := ms.TrackToken("a1")
	if !ok {
		t.Fatal("a1 not staged")
	}

	manifestTok, err := ms.ManifestToken()
	if err != nil {
		t.Fatalf("ManifestToken: %v", err)
	}
	res, err := relay.Open(context.Background(), manifestTok)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	manifest, _ := io.ReadAll(res.Body)
	if !strings.HasPrefix(string(manifest), "#EXTM3U") {
		t.Fatalf("manifest = %q, want an HLS playlist", manifest)
	}
	if !strings.Contains(string(manifest), RelayPrefix+v1Tok) || !strings.Contains(string(manifest), RelayPrefix+a1Tok) {
		t.Fatalf("manifest does not reference the per-track relay URLs:\n%s", manifest)
	}

	tres, err := relay.Open(context.Background(), v1Tok)
	if err != nil {
		t.Fatalf("open track: %v", err)
	}
	defer func() { _ = tres.Body.Close() }()
	body, _ := io.ReadAll(tres.Body)
	if string(body) != "track-bytes" {
		t.Fatalf("track body = %q, want track-bytes", body)
	}

	if err := sink.Finalize(context.Background(), sess); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if _, err := relay.Open(context.Background(), manifestTok); err == nil {
		t.Fatal("manifest token still valid after finalize")
	}
}

func TestMediaServerSinkSkipsCarriedInTracks(t *testing.T) {
	srv := plainServer(t)
	defer srv.Close()
	relay := NewRelay()
	opener := NewOpener(nil, nil, nil)
	factory := &MediaServerSinkFactory{Relay: relay, Opener: opener, TTL: time.Minute}
	sess := &Session{ID: "del-3", Context: corev1.DeliveryContext{Provider: "p", NativeId: "n"}}

	src := plainSource(srv.URL)
	carriedIn := &corev1.Track{
		Id:       "sub1",
		Media:    &corev1.Track_Subtitle{Subtitle: &corev1.SubtitleTrack{Format: "webvtt", Language: "en"}},
		Delivery: &corev1.TrackDelivery{CarriedIn: "v1"}, // no location: no independent bytes
	}
	tracks := append(src.Tracks, carriedIn)

	sink, err := factory.NewSink(context.Background(), sess, tracks)
	if err != nil {
		t.Fatalf("NewSink: %v", err)
	}
	ms := sink.(*MediaServerSink)
	for _, tr := range tracks {
		if _, err := sink.Deliver(context.Background(), sess, tr, nil); err != nil {
			t.Fatalf("Deliver: %v", err)
		}
	}
	if _, ok := ms.TrackToken("sub1"); ok {
		t.Fatal("carried-in track must not stage a relay ticket")
	}
	if _, err := ms.ManifestToken(); err != nil {
		t.Fatalf("ManifestToken: %v", err)
	}
}

func TestMediaServerSinkPullNotesActivity(t *testing.T) {
	now := time.Now()
	sess := &Session{ID: "del-2", Context: corev1.DeliveryContext{Provider: "p", NativeId: "n"}, now: func() time.Time { return now }}
	srv := plainServer(t)
	defer srv.Close()
	relay := NewRelay()
	opener := NewOpener(nil, nil, nil)
	factory := &MediaServerSinkFactory{Relay: relay, Opener: opener, TTL: time.Minute}
	sink, err := factory.NewSink(context.Background(), sess, plainSource(srv.URL).Tracks)
	if err != nil {
		t.Fatalf("NewSink: %v", err)
	}
	ms := sink.(*MediaServerSink)
	track := plainSource(srv.URL).Tracks[0]
	if _, err := sink.Deliver(context.Background(), sess, track, nil); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	tok, _ := ms.TrackToken(track.GetId())

	sess.mu.Lock()
	before := sess.lastProgress
	sess.mu.Unlock()

	now = now.Add(time.Minute)
	if _, err := relay.Open(context.Background(), tok); err != nil {
		t.Fatalf("open: %v", err)
	}

	sess.mu.Lock()
	after := sess.lastProgress
	sess.mu.Unlock()
	if !after.After(before) {
		t.Fatalf("pull did not refresh activity: before=%v after=%v", before, after)
	}
}

// TestMediaServerSinkLivenessIsTrafficBased pins the generic-player liveness
// rule: a media-server play session is kept alive by pull activity (a generic
// player never heartbeats), and is evicted only when it goes idle past the
// window with no heartbeat and no pull.
func TestMediaServerSinkLivenessIsTrafficBased(t *testing.T) {
	now := time.Now()
	srv := plainServer(t)
	defer srv.Close()
	relay := NewRelay()
	opener := NewOpener(nil, nil, nil)

	e := New(Options{
		SessionTTL:        time.Hour,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatGrace:    5 * time.Second,
		Now:               func() time.Time { return now },
		RecordJob:         func(*corev1.Job) {},
		SourceResolver:    &fakeResolver{source: plainSource(srv.URL)},
		Opener:            opener,
		SinkFactory:       &MediaServerSinkFactory{Relay: relay, Opener: opener, TTL: time.Minute},
	})
	defer e.Close()

	s, err := e.Start(context.Background(), StartRequest{
		Goal: GoalPlay, MemberUserID: "u", Provider: "jellyfin", AccountID: "a", NativeID: "item-1", Sink: "mediaserver",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ms := s.Sink.(*MediaServerSink)
	trackTok, ok := ms.TrackToken("v1")
	if !ok {
		t.Fatal("v1 not staged")
	}

	// Pull activity keeps the session alive past the heartbeat window.
	now = now.Add(40 * time.Second)
	if _, err := relay.Open(context.Background(), trackTok); err != nil {
		t.Fatalf("open: %v", err)
	}
	e.sweep()
	if _, ok := e.Get(s.ID); !ok {
		t.Fatal("active media-server session evicted despite pull activity")
	}

	// Idle past the window with no pull: evicted.
	now = now.Add(40 * time.Second)
	e.sweep()
	if _, ok := e.Get(s.ID); ok {
		t.Fatal("idle media-server session not evicted")
	}
}
