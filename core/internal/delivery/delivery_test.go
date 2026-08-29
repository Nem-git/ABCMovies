package delivery

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// fakeResolver returns a canned static, whole-mux manifest.
type fakeResolver struct {
	source *corev1.MediaSource
	err    error
}

func (f *fakeResolver) ProduceSources(ctx context.Context, provider, accountID, nativeID string) (*corev1.MediaSource, error) {
	return f.source, f.err
}

func staticSource() *corev1.MediaSource {
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_PER_TRACK,
		Tracks: []*corev1.Track{
			{Id: "v1", Media: &corev1.Track_Video{}},
			{Id: "a1", Media: &corev1.Track_Audio{}},
		},
	}
}

// wholeMuxSource is the realistic download source shape: a muxed container
// fetched as a unit (§6.2). Container "mkv" carries the video and audio.
func wholeMuxSource() *corev1.MediaSource {
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_WHOLE_MUX,
		Tracks: []*corev1.Track{
			{
				Id: "c1",
				Media: &corev1.Track_Video{
					Video: &corev1.VideoTrack{Codec: "h264"},
				},
				Delivery: &corev1.TrackDelivery{Locations: []string{"https://cdn/x.mkv"}},
			},
			{
				Id:       "a1",
				Media:    &corev1.Track_Audio{Audio: &corev1.AudioTrack{Codec: "aac"}},
				Delivery: &corev1.TrackDelivery{CarriedIn: "c1"},
			},
		},
	}
}

// recordingSink records deliveries and finalize/abort calls.
type recordingSink struct {
	delivered []string
	finalized bool
	aborted   bool
}

func (r *recordingSink) Deliver(ctx context.Context, s *Session, track *corev1.Track, body io.Reader) (int64, error) {
	r.delivered = append(r.delivered, track.GetId())
	return 0, nil
}

func (r *recordingSink) Finalize(ctx context.Context, s *Session) error {
	r.finalized = true
	return nil
}
func (r *recordingSink) Abort(ctx context.Context, s *Session) { r.aborted = true }

type fakeSinkFactory struct {
	sinks map[string]*recordingSink
}

func (f *fakeSinkFactory) NewSink(ctx context.Context, s *Session, tracks []*corev1.Track) (Sink, error) {
	r := &recordingSink{}
	if f.sinks == nil {
		f.sinks = map[string]*recordingSink{}
	}
	f.sinks[s.ID] = r
	return r, nil
}

func newTestEngine(opts Options) (*Engine, *fakeResolver, *fakeSinkFactory) {
	res := &fakeResolver{source: staticSource()}
	fac := &fakeSinkFactory{}
	opts.SourceResolver = res
	opts.SinkFactory = fac
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return New(opts), res, fac
}

func TestStartCreatesSession(t *testing.T) {
	now := time.Now()
	e, _, fac := newTestEngine(Options{
		SessionTTL: 24 * time.Hour,
		Now:        func() time.Time { return now },
		RecordJob:  func(_ *corev1.Job) {},
	})
	if fac == nil {
		t.Fatal("no factory")
	}
	defer e.Close()

	s, err := e.Start(context.Background(), StartRequest{
		Goal:         GoalPlay,
		MemberUserID: "u1",
		Provider:     "jellyfin",
		AccountID:    "acc1",
		NativeID:     "item1",
		Sink:         "device",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if s.Status != StatusRunning {
		t.Errorf("status = %s, want running", s.Status)
	}
	if s.Context.GetMemberUserId() != "u1" {
		t.Errorf("member = %q", s.Context.GetMemberUserId())
	}
	if s.Context.GetProvider() != "jellyfin" {
		t.Errorf("provider = %q", s.Context.GetProvider())
	}
	if fac.sinks[s.ID] == nil {
		t.Errorf("sink was not created for session %q", s.ID)
	}
}

func TestStartCreatesDistinctSessions(t *testing.T) {
	// Per TECHNICAL-DECISIONS.md §1.30, Start is a plain non-idempotent
	// "create": each call makes a fresh session (there is no client-supplied
	// idempotency key; double-start is bounded structurally by the cap).
	e, _, _ := newTestEngine(Options{
		SessionTTL: 24 * time.Hour,
		Now:        time.Now,
		RecordJob:  func(_ *corev1.Job) {},
	})
	defer e.Close()
	req := StartRequest{
		Goal:         GoalPlay,
		MemberUserID: "u1",
		Provider:     "jellyfin",
		AccountID:    "acc1",
		NativeID:     "item1",
		Sink:         "device",
	}
	a, err := e.Start(context.Background(), req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	b, err := e.Start(context.Background(), req)
	if err != nil {
		t.Fatalf("second Start: %v", err)
	}
	if a.ID == b.ID {
		t.Errorf("Start unexpectedly returned the same session: %q", a.ID)
	}
	if e.LiveSessions() != 2 {
		t.Errorf("LiveSessions = %d, want 2", e.LiveSessions())
	}
}

func TestStartRejectsInvalidManifest(t *testing.T) {
	res := &fakeResolver{source: &corev1.MediaSource{}}
	e := New(Options{SessionTTL: time.Hour, Now: time.Now, RecordJob: func(*corev1.Job) {}, SourceResolver: res})
	defer e.Close()
	_, err := e.Start(context.Background(), StartRequest{
		Goal: GoalPlay, MemberUserID: "u",
		Provider: "jellyfin", AccountID: "a", Sink: "device",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid manifest") {
		t.Fatalf("expected invalid manifest error, got %v", err)
	}
}

// drmSource is a schema-valid PER_TRACK manifest whose video track is
// DRM-encrypted — the contract allows it, the engine must not deliver it.
func drmSource() *corev1.MediaSource {
	src := staticSource()
	src.Tracks[0].Delivery = &corev1.TrackDelivery{
		Drm: &corev1.TrackDrm{System: "org.w3.clearkey", LicenseUrl: "https://license.example/"},
	}
	return src
}

func TestRejectUnsupportedDRM(t *testing.T) {
	if err := RejectUnsupportedDRM(nil); err != nil {
		t.Fatalf("nil manifest refused: %v", err)
	}
	if err := RejectUnsupportedDRM(staticSource()); err != nil {
		t.Fatalf("clean manifest refused: %v", err)
	}
	err := RejectUnsupportedDRM(drmSource())
	if err == nil {
		t.Fatal("DRM-encrypted manifest accepted, want refusal")
	}
	if !strings.Contains(err.Error(), `track "v1"`) || !strings.Contains(err.Error(), "not supported in v1") {
		t.Fatalf("refusal must name the offending track and v1's missing license path, got %q", err.Error())
	}
}

func TestStartRefusesDRMTrackLoudly(t *testing.T) {
	for _, goal := range []Goal{GoalPlay, GoalDownload} {
		var recorded []*corev1.Job
		e := New(Options{
			SessionTTL:     time.Hour,
			Now:            time.Now,
			RecordJob:      func(j *corev1.Job) { recorded = append(recorded, j) },
			SourceResolver: &fakeResolver{source: drmSource()},
			SinkFactory:    &DeviceSinkFactory{Relay: NewRelay()},
		})
		_, err := e.Start(context.Background(), StartRequest{
			Goal: goal, MemberUserID: "u",
			Provider: "jellyfin", AccountID: "a", NativeID: "item1", Sink: "device",
		})
		if err == nil || !strings.Contains(err.Error(), "DRM-encrypted") {
			t.Fatalf("%s: expected DRM refusal, got %v", goal, err)
		}
		if len(recorded) != 0 {
			t.Fatalf("%s: a refused session must not announce a job, got %d", goal, len(recorded))
		}
		if e.LiveSessions() != 0 {
			t.Fatalf("%s: LiveSessions = %d, want 0", goal, e.LiveSessions())
		}
		e.Close()
	}
}

func TestHeartbeatBoundaries(t *testing.T) {
	now := time.Now()
	e, _, _ := newTestEngine(Options{
		SessionTTL:        24 * time.Hour,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatGrace:    90 * time.Second,
		Now:               func() time.Time { return now },
		RecordJob:         func(_ *corev1.Job) {},
	})
	defer e.Close()
	s, _ := e.Start(context.Background(), StartRequest{
		Goal: GoalPlay, MemberUserID: "u",
		Provider: "jellyfin", AccountID: "a", Sink: "device",
	})
	if err := e.Heartbeat(s.ID); err != nil {
		t.Fatalf("Heartbeat active: %v", err)
	}
	if err := e.Complete(s.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if err := e.Heartbeat(s.ID); err == nil {
		t.Errorf("heartbeat on completed session should fail")
	}
	if err := e.Heartbeat("nope"); err == nil {
		t.Errorf("heartbeat on unknown session should fail")
	}
}

func TestPlayHeartbeatTimeoutEvicts(t *testing.T) {
	now := time.Now()
	var recorded []*corev1.Job
	e, _, _ := newTestEngine(Options{
		SessionTTL:        24 * time.Hour,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatGrace:    5 * time.Second,
		Now:               func() time.Time { return now },
		RecordJob:         func(j *corev1.Job) { recorded = append(recorded, j) },
	})
	defer e.Close()

	s, _ := e.Start(context.Background(), StartRequest{
		Goal: GoalPlay, MemberUserID: "u",
		Provider: "jellyfin", AccountID: "a", Sink: "device",
	})
	// Advance time past heartbeat timeout without a heartbeat.
	now = now.Add(40 * time.Second)
	e.sweep()
	if _, ok := e.Get(s.ID); ok {
		t.Errorf("session still present after heartbeat timeout")
	}
	if e.LiveSessions() != 0 {
		t.Errorf("LiveSessions = %d, want 0", e.LiveSessions())
	}
	// The eviction must be recorded as a failed job.
	if len(recorded) == 0 {
		t.Fatalf("no job recorded for evicted session")
	}
	last := recorded[len(recorded)-1]
	if last.Status != corev1.JobStatus_JOB_STATUS_FAILED {
		t.Errorf("evicted job status = %s, want failed", last.Status)
	}
}

func TestTTLExpiryEvicts(t *testing.T) {
	now := time.Now()
	e, res, _ := newTestEngine(Options{
		SessionTTL: 10 * time.Minute,
		Now:        func() time.Time { return now },
		RecordJob:  func(_ *corev1.Job) {},
	})
	res.source = wholeMuxSource()
	defer e.Close()
	s, _ := e.Start(context.Background(), StartRequest{
		Goal: GoalDownload, MemberUserID: "u",
		Provider: "jellyfin", AccountID: "a", Sink: "disk",
	})
	// Keep heartbeating but exceed the TTL.
	for i := 0; i < 3; i++ {
		now = now.Add(5 * time.Minute)
		_ = e.Progress(s.ID, 10)
		e.sweep()
	}
	if _, ok := e.Get(s.ID); ok {
		t.Errorf("session survived beyond TTL")
	}
}

func TestProgressKeepsDownloadAlive(t *testing.T) {
	now := time.Now()
	e, res, _ := newTestEngine(Options{
		SessionTTL: 10 * time.Minute,
		Now:        func() time.Time { return now },
		RecordJob:  func(_ *corev1.Job) {},
	})
	res.source = wholeMuxSource()
	defer e.Close()
	s, _ := e.Start(context.Background(), StartRequest{
		Goal: GoalDownload, MemberUserID: "u",
		Provider: "jellyfin", AccountID: "a", Sink: "disk",
	})
	now = now.Add(9 * time.Minute)
	if err := e.Progress(s.ID, 50); err != nil {
		t.Fatalf("Progress: %v", err)
	}
	e.sweep()
	if _, ok := e.Get(s.ID); !ok {
		t.Errorf("download evicted while progressing")
	}
}

func TestRevokeAccountEndsSessions(t *testing.T) {
	var recorded []*corev1.Job
	e, res, fac := newTestEngine(Options{
		SessionTTL: 24 * time.Hour,
		Now:        time.Now,
		RecordJob:  func(j *corev1.Job) { recorded = append(recorded, j) },
	})
	res.source = wholeMuxSource()
	defer e.Close()
	req := func() StartRequest {
		return StartRequest{
			Goal: GoalDownload, MemberUserID: "u1",
			Provider: "jellyfin", AccountID: "acc1", NativeID: "i", Sink: "disk",
		}
	}
	s1, _ := e.Start(context.Background(), req())
	s2, _ := e.Start(context.Background(), req())
	// A different member on the same account must survive.
	s3, _ := e.Start(context.Background(), func() StartRequest {
		r := req()
		r.MemberUserID = "u2"
		return r
	}())

	if n := e.RevokeAccount("jellyfin", "acc1", "u1"); n != 2 {
		t.Errorf("revoked %d sessions, want 2", n)
	}
	if got, _ := e.Get(s1.ID); got == nil || got.Status != StatusRevoked {
		t.Errorf("s1 not revoked (status=%v)", got.Status)
	}
	if got, _ := e.Get(s2.ID); got == nil || got.Status != StatusRevoked {
		t.Errorf("s2 not revoked (status=%v)", got.Status)
	}
	if s3, _ := e.Get(s3.ID); s3 == nil || s3.Status != StatusRunning {
		t.Errorf("s3 (other member) should survive, got %v", s3)
	}
	if !fac.sinks[s1.ID].aborted {
		t.Errorf("revoked session's sink was not aborted")
	}
	var sawRevoked bool
	for _, j := range recorded {
		if j.GetId() == s1.ID && j.Error == "account revoked" {
			sawRevoked = true
		}
	}
	if !sawRevoked {
		t.Errorf("revocation not recorded as job error")
	}
}

func TestStartRejectsMissingFields(t *testing.T) {
	e := New(Options{SessionTTL: time.Hour, RecordJob: func(*corev1.Job) {}})
	defer e.Close()
	base := StartRequest{
		Goal: GoalPlay, MemberUserID: "u",
		Provider: "jellyfin", AccountID: "a", Sink: "device",
	}
	cases := []struct {
		name string
		edit func(*StartRequest)
	}{
		{"bad goal", func(r *StartRequest) { r.Goal = "bogus" }},
		{"no provider", func(r *StartRequest) { r.Provider = "" }},
		{"no account", func(r *StartRequest) { r.AccountID = "" }},
		{"no member", func(r *StartRequest) { r.MemberUserID = "" }},
		{"no sink", func(r *StartRequest) { r.Sink = "" }},
	}
	for _, c := range cases {
		r := base
		c.edit(&r)
		if _, err := e.Start(context.Background(), r); err == nil {
			t.Errorf("%s: expected error", c.name)
		}
	}
}

func TestCode(t *testing.T) {
	if c := Code(errInvalid("x")); c != 3 {
		t.Errorf("invalid code = %d", c)
	}
	if c := Code(errNotFound("x")); c != 5 {
		t.Errorf("notfound code = %d", c)
	}
	if c := Code(errors.New("boom")); c != 2 {
		t.Errorf("unknown code = %d", c)
	}
	if c := Code(nil); c != 0 {
		t.Errorf("nil code = %d", c)
	}
}

func TestWatchLoopEvictsAndCloseReturns(t *testing.T) {
	e := New(Options{
		SessionTTL:        30 * time.Millisecond,
		HeartbeatInterval: 10 * time.Millisecond,
		HeartbeatGrace:    10 * time.Millisecond,
		RecordJob:         func(*corev1.Job) {},
		SourceResolver:    &fakeResolver{source: staticSource()},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Watch(ctx)

	s, err := e.Start(context.Background(), StartRequest{
		Goal: GoalPlay, MemberUserID: "u",
		Provider: "jellyfin", AccountID: "a", Sink: "device",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// No heartbeat; the TTL is shorter than the watchdog cadence, so the
	// expired session is evicted.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := e.Get(s.ID); !ok {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, ok := e.Get(s.ID); ok {
		t.Fatalf("session was not evicted by the watchdog")
	}
	// Close must return promptly even while Watch is running.
	done := make(chan struct{})
	go func() { e.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Close blocked")
	}
}
