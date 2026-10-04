package delivery

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/policy"
)

func TestStartEnforcesConcurrentStreamsCap(t *testing.T) {
	now := time.Now()
	e, res, _ := newTestEngine(Options{
		SessionTTL:        24 * time.Hour,
		InstancePolicy: policy.Set{"concurrentStreams": "2"},
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

	// A different account on the same member draws on the same member-level
	// budget: two open sessions across accounts refuse a third anywhere.
	req2 := req
	req2.AccountID = "acc2"
	if _, err := e.Start(context.Background(), req2); err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("expected member aggregate to refuse a third start, got %v", err)
	}

	// A different member on an unoccupied account still starts: the quota
	// only binds the member's own sessions (instance policy: 2) or sessions
	// through that account (2 here), and u2 on acc2 trips neither.
	req3 := req
	req3.AccountID = "acc2"
	req3.MemberUserID = "u2"
	if _, err := e.Start(context.Background(), req3); err != nil {
		t.Fatalf("different member on another account: %v", err)
	}
}

func TestStartHonorsAccountOverrideUnderMemberCap(t *testing.T) {
	now := time.Now()
	override := policy.Set{"concurrentStreams": "1"}
	e, res, _ := newTestEngine(Options{
		SessionTTL: 24 * time.Hour,
		Now:        func() time.Time { return now },
		RecordJob:  func(*corev1.Job) {},
		AccountConstraints: func(_ context.Context, _, _ string) (policy.Set, policy.Set, error) {
			return override, nil, nil
		},
	})
	res.source = wholeMuxSource()
	defer e.Close()

	req := StartRequest{Goal: GoalDownload, MemberUserID: "u", Provider: "jellyfin", AccountID: "a", Sink: "disk"}
	if _, err := e.Start(context.Background(), req); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := e.Start(context.Background(), req); err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("account override to 1 should refuse a second start, got %v", err)
	}
}

func TestStartHonorsProviderCapOverPolicy(t *testing.T) {
	now := time.Now()
	e, res, _ := newTestEngine(Options{
		SessionTTL: 24 * time.Hour,
		Now:        func() time.Time { return now },
		RecordJob:  func(*corev1.Job) {},
		AccountConstraints: func(_ context.Context, _, _ string) (policy.Set, policy.Set, error) {
			return nil, policy.Set{"concurrentStreams": "1"}, nil
		},
	})
	res.source = wholeMuxSource()
	defer e.Close()

	req := StartRequest{Goal: GoalDownload, MemberUserID: "u", Provider: "jellyfin", AccountID: "a", Sink: "disk"}
	if _, err := e.Start(context.Background(), req); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := e.Start(context.Background(), req); err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("provider cap of 1 should refuse a second start, got %v", err)
	}
}

func TestCompleteFreesCapSlot(t *testing.T) {
	now := time.Now()
	e, res, _ := newTestEngine(Options{
		SessionTTL:        24 * time.Hour,
		InstancePolicy: policy.Set{"concurrentStreams": "1"},
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
