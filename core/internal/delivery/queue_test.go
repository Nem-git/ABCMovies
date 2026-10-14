package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/policy"
)

// queueTestEngine builds an engine with a one-stream cap and a controllable
// clock, so a test can drive queueing, promotion, and expiry deterministically.
func queueTestEngine(t *testing.T) (*Engine, *fakeResolver, *fakeSinkFactory, *time.Time) {
	t.Helper()
	now := time.Now()
	e, res, fac := newTestEngine(Options{
		SessionTTL:     config.DefaultSessionTTL,
		InstancePolicy: policy.Set{policy.KeyConcurrentStreams: "1"},
		Now:            func() time.Time { return now },
		RecordJob:      func(*corev1.Job) {},
	})
	res.source = wholeMuxSource()
	return e, res, fac, &now
}

func downloadStart(member string) StartRequest {
	return StartRequest{
		Goal:         GoalDownload,
		MemberUserID: member,
		Provider:     "jellyfin",
		AccountID:    "acc1",
		NativeID:     "movie-1",
		Sink:         "relay",
	}
}

// A download at the seat cap queues: the start succeeds with status QUEUED
// and a position, not an error (PLAN.md §6.5).
func TestDownloadQueuesAtCap(t *testing.T) {
	e, _, _, _ := queueTestEngine(t)
	defer e.Close()

	if _, err := e.Start(context.Background(), downloadStart("u1")); err != nil {
		t.Fatalf("first download: %v", err)
	}
	_, err := e.Start(context.Background(), downloadStart("u1"))
	if err == nil {
		t.Fatal("second download should queue, not start")
	}
	var qerr *ErrQueued
	if !errors.As(err, &qerr) {
		t.Fatalf("error = %T (%v), want ErrQueued", err, err)
	}
	job := qerr.Job()
	if job.GetStatus() != corev1.JobStatus_JOB_STATUS_QUEUED {
		t.Errorf("status = %v, want QUEUED", job.GetStatus())
	}
	if job.GetQueued().GetPosition() != 1 {
		t.Errorf("position = %d, want 1", job.GetQueued().GetPosition())
	}
}

// A play at the seat cap still fails loudly — play gets a priority path,
// never a queue (PLAN.md §6.5).
func TestPlayRefusesAtCap(t *testing.T) {
	e, _, _, _ := queueTestEngine(t)
	defer e.Close()

	if _, err := e.Start(context.Background(), reauthStart()); err != nil {
		t.Fatalf("first play: %v", err)
	}
	_, err := e.Start(context.Background(), reauthStart())
	if err == nil {
		t.Fatal("second play should refuse")
	}
	if got := Reason(err); got != RefusalAtCapacity {
		t.Errorf("reason = %q, want at-capacity", got)
	}
	if _, ok := err.(*ErrQueued); ok {
		t.Error("a play must never queue")
	}
}

// A queued download starts when the running one completes: the seat-release
// hook promotes the next queued download (PLAN.md §6.5).
func TestQueuedDownloadPromotesOnComplete(t *testing.T) {
	e, _, _, _ := queueTestEngine(t)
	defer e.Close()

	if _, err := e.Start(context.Background(), downloadStart("u1")); err != nil {
		t.Fatalf("first download: %v", err)
	}
	_, err := e.Start(context.Background(), downloadStart("u1"))
	if err == nil {
		t.Fatal("second download should queue")
	}
	var qerr *ErrQueued
	if !errors.As(err, &qerr) {
		t.Fatalf("queue error = %T, want ErrQueued", err)
	}
	queuedID := qerr.Job().GetId()

	// Complete the running download; the queued one should promote.
	// Snapshot the ids first: promoting mutates e.sessions, and iterating a
	// map while it changes would also complete the freshly promoted session.
	var ids []string
	for id := range e.sessions {
		ids = append(ids, id)
	}
	for _, id := range ids {
		if err := e.Complete(context.Background(), id); err != nil {
			t.Fatalf("complete: %v", err)
		}
	}
	sess, ok := e.Get(queuedID)
	if !ok {
		t.Fatal("queued download did not promote to a session")
	}
	if sess.Status != StatusRunning {
		t.Errorf("status = %s, want running", sess.Status)
	}
}

// A retry with the same idempotency key returns the same queued job instead
// of enqueuing a second entry (PLAN.md §9.1).
func TestIdempotencyKeyReturnsSameQueuedJob(t *testing.T) {
	e, _, _, _ := queueTestEngine(t)
	defer e.Close()

	// Fill the cap so the keyed start queues.
	if _, err := e.Start(context.Background(), downloadStart("u1")); err != nil {
		t.Fatalf("first start: %v", err)
	}
	req := downloadStart("u1")
	req.IdempotencyKey = "key-1"
	_, err := e.Start(context.Background(), req)
	if err == nil {
		t.Fatal("second start should queue")
	}
	var qerr *ErrQueued
	if !errors.As(err, &qerr) {
		t.Fatalf("error = %T, want ErrQueued", err)
	}
	queuedID := qerr.Job().GetId()
	if queuedID == "" {
		t.Fatal("queued job carries no id")
	}

	// A retry with the same key returns the same queued job, not a second
	// queue entry.
	_, err = e.Start(context.Background(), req)
	if err == nil {
		t.Fatal("retry should return the queued job")
	}
	var retry *ErrQueued
	if !errors.As(err, &retry) {
		t.Fatalf("retry error = %T, want ErrQueued", err)
	}
	if retry.Job().GetId() != queuedID {
		t.Errorf("retry id = %q, want %q", retry.Job().GetId(), queuedID)
	}
	if len(e.queue["u1"]) != 1 {
		t.Errorf("queue length = %d, want 1", len(e.queue["u1"]))
	}
}

// A queued download can be cancelled: it leaves the queue and its job is
// marked CANCELLED (PLAN.md §6.5).
func TestCancelQueuedDownload(t *testing.T) {
	e, _, _, _ := queueTestEngine(t)
	defer e.Close()

	if _, err := e.Start(context.Background(), downloadStart("u1")); err != nil {
		t.Fatalf("first download: %v", err)
	}
	_, err := e.Start(context.Background(), downloadStart("u1"))
	if err == nil {
		t.Fatal("second download should queue")
	}
	var qerr *ErrQueued
	if !errors.As(err, &qerr) {
		t.Fatalf("queue error = %T, want ErrQueued", err)
	}
	id := qerr.Job().GetId()

	sess, err := e.Cancel(id)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if sess != nil {
		t.Error("a queued job has no session")
	}
	if len(e.queue["u1"]) != 0 {
		t.Errorf("queue length after cancel = %d, want 0", len(e.queue["u1"]))
	}
}

// A running download can be cancelled: its session ends and the job is
// marked CANCELLED (PLAN.md §6.5).
func TestCancelRunningDownload(t *testing.T) {
	e, _, _, _ := queueTestEngine(t)
	defer e.Close()

	sess, err := e.Start(context.Background(), downloadStart("u1"))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	cancelled, err := e.Cancel(sess.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled == nil {
		t.Fatal("cancel of a running download should return the session")
	}
	if cancelled.Status != StatusCancelled {
		t.Errorf("status = %s, want cancelled", cancelled.Status)
	}
}

// A download starts immediately when the system is idle — no queue wait.
// This is the fast-path: when there is capacity, downloads start right away.
func TestDownloadStartsImmediatelyWhenIdle(t *testing.T) {
	e, _, _, _ := queueTestEngine(t)
	defer e.Close()

	sess, err := e.Start(context.Background(), downloadStart("u1"))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if sess == nil {
		t.Fatal("start returned nil session")
	}
	if sess.Status != StatusRunning {
		t.Errorf("status = %s, want running", sess.Status)
	}
	if len(e.queue["u1"]) != 0 {
		t.Errorf("queue length = %d, want 0 (fast-path should not queue)", len(e.queue["u1"]))
	}
}

// A download that waits longer than the queue TTL is failed, not left
// forever (PLAN.md §6.5).
func TestQueuedDownloadExpiresAfterTTL(t *testing.T) {
	e, _, _, now := queueTestEngine(t)
	defer e.Close()
	e.queueTTL = time.Hour

	if _, err := e.Start(context.Background(), downloadStart("u1")); err != nil {
		t.Fatalf("first download: %v", err)
	}
	_, err := e.Start(context.Background(), downloadStart("u1"))
	if err == nil {
		t.Fatal("second download should queue")
	}
	if len(e.queue["u1"]) != 1 {
		t.Fatalf("queue length = %d, want 1", len(e.queue["u1"]))
	}

	// Advance past the TTL and sweep.
	*now = now.Add(2 * time.Hour)
	e.sweep()
	if len(e.queue["u1"]) != 0 {
		t.Errorf("queue length after TTL = %d, want 0", len(e.queue["u1"]))
	}
}
