package delivery

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

func TestStartEnforcesConcurrentStreamsCap(t *testing.T) {
	now := time.Now()
	e, res, _ := newTestEngine(Options{
		SessionTTL:        24 * time.Hour,
		ConcurrentStreams: 2,
		Now:               func() time.Time { return now },
		RecordJob:         func(*corev1.Job) {},
	})
	res.source = wholeMuxSource()
	defer e.Close()

	req := StartRequest{
		Goal: GoalDownload, MemberUserID: "u1",
		Provider: "jellyfin", AccountID: "acc1", Sink: "disk",
	}
	if _, err := e.Start(context.Background(), req); err != nil {
		t.Fatalf("session 1: %v", err)
	}
	if _, err := e.Start(context.Background(), req); err != nil {
		t.Fatalf("session 2: %v", err)
	}
	// Third simultaneous start for the same account+member hits the cap.
	_, err := e.Start(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("expected cap rejection, got %v", err)
	}

	// A different account on the same member is not capped together.
	req2 := req
	req2.AccountID = "acc2"
	if _, err := e.Start(context.Background(), req2); err != nil {
		t.Fatalf("session on another account: %v", err)
	}
}

func TestCompleteFreesCapSlot(t *testing.T) {
	now := time.Now()
	e, res, _ := newTestEngine(Options{
		SessionTTL:        24 * time.Hour,
		ConcurrentStreams: 1,
		Now:               func() time.Time { return now },
		RecordJob:         func(*corev1.Job) {},
	})
	res.source = wholeMuxSource()
	defer e.Close()

	req := StartRequest{Goal: GoalDownload, MemberUserID: "u", Provider: "jellyfin", AccountID: "a", Sink: "disk"}
	s1, err := e.Start(context.Background(), req)
	if err != nil {
		t.Fatalf("session 1: %v", err)
	}
	if _, err := e.Start(context.Background(), req); err == nil {
		t.Fatal("second start should hit cap of 1")
	}
	if err := e.Complete(s1.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, err := e.Start(context.Background(), req); err != nil {
		t.Fatalf("start after complete should succeed, got %v", err)
	}
}
