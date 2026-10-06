package delivery

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/policy"
)

func TestStartEnforcesConcurrentStreamsCap(t *testing.T) {
	now := time.Now()
	e, res, _ := newTestEngine(Options{
		SessionTTL:     24 * time.Hour,
		InstancePolicy: policy.Set{"concurrentStreams": "2"},
		Now:            func() time.Time { return now },
		RecordJob:      func(*corev1.Job) {},
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
		SessionTTL:     24 * time.Hour,
		InstancePolicy: policy.Set{"concurrentStreams": "1"},
		Now:            func() time.Time { return now },
		RecordJob:      func(*corev1.Job) {},
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

// Lowering an account's declared cap to N ends the account's excess sessions,
// newest-first-killing order being "oldest loses": the longest-running
// session is the one ended, the freshest start is the one kept. And a cap
// change that does not breach the running count ends nothing.
func TestApplyAccountCapEnforcesOnCapChange(t *testing.T) {
	now := time.Now()
	currentCap := 3
	e, res, _ := newTestEngine(Options{
		SessionTTL:     24 * time.Hour,
		InstancePolicy: policy.Set{"concurrentStreams": "3"},
		Now:            func() time.Time { return now },
		RecordJob:      func(*corev1.Job) {},
		AccountConstraints: func(ctx context.Context, provider, accountID string) (policy.Set, policy.Set, error) {
			return nil, policy.Set{"concurrentStreams": strconv.Itoa(currentCap)}, nil
		},
	})
	res.source = wholeMuxSource()
	defer e.Close()

	start := func(member string) *Session {
		s, err := e.Start(context.Background(), StartRequest{
			Goal: GoalDownload, MemberUserID: member,
			Provider: "jellyfin", AccountID: "acc1", Sink: "disk",
		})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		return s
	}
	u1a := start("u1")
	now = now.Add(time.Minute)
	u2a := start("u2")
	now = now.Add(time.Minute)
	u1b := start("u1")

	// New-sessions-only behaviour must end nothing.
	currentCap = 1
	if killed, err := e.ApplyAccountCap(context.Background(), "jellyfin", "acc1", false); err != nil || killed != 0 {
		t.Fatalf("new-sessions-only: killed=%d err=%v, want 0 sessions", killed, err)
	}

	// Enforcing ends the two oldest; the last start survives.
	killed, err := e.ApplyAccountCap(context.Background(), "jellyfin", "acc1", true)
	if err != nil {
		t.Fatalf("ApplyAccountCap: %v", err)
	}
	if killed != 2 {
		t.Fatalf("killed = %d, want 2", killed)
	}
	if got, _ := e.Get(u1a.ID); got == nil || got.Status != StatusRevoked {
		t.Errorf("oldest session u1a should be revoked, got %v", got)
	}
	if got, _ := e.Get(u2a.ID); got == nil || got.Status != StatusRevoked {
		t.Errorf("second-oldest session u2a should be revoked, got %v", got)
	}
	if got, _ := e.Get(u1b.ID); got == nil || got.Status != StatusRunning {
		t.Errorf("newest session u1b should survive, got %v", got)
	}

	// The cap the account now enforces is what admission will allow next:
	// the running session through it is enough — a fourth queued session is
	// refused, the account's one running session must not be punished again.
	if killed, _ := e.ApplyAccountCap(context.Background(), "jellyfin", "acc1", true); killed != 0 {
		t.Errorf("second enforcement killed %d more, want 0", killed)
	}
}

// RevokeOthersOnAccount is the narrowing path: whoever lost access loses
// their live sessions on that account; the owner, newly-added members, and
// the account's other sessions keep theirs.
func TestRevokeOthersOnAccountKillsOnlyLoseAccessMembers(t *testing.T) {
	var recorded []*corev1.Job
	e, res, _ := newTestEngine(Options{
		SessionTTL: 24 * time.Hour,
		RecordJob:  func(j *corev1.Job) { recorded = append(recorded, j) },
	})
	res.source = wholeMuxSource()
	defer e.Close()

	start := func(member, account string) *Session {
		s, err := e.Start(context.Background(), StartRequest{
			Goal: GoalDownload, MemberUserID: member,
			Provider: "jellyfin", AccountID: account, Sink: "disk",
		})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		return s
	}
	owner := start("owner", "acc1")
	dropped := start("dropped", "acc1")
	keptOnOther := start("dropped", "acc2")
	if n := e.RevokeOthersOnAccount("jellyfin", "acc1", []string{"owner"}); n != 1 {
		t.Fatalf("revoked %d, want 1", n)
	}
	if got, _ := e.Get(owner.ID); got == nil || got.Status != StatusRunning {
		t.Errorf("owner's session should survive, got %v", got)
	}
	if got, _ := e.Get(dropped.ID); got == nil || got.Status != StatusRevoked {
		t.Errorf("dropped member's session should be revoked, got %v", got)
	}
	if got, _ := e.Get(keptOnOther.ID); got == nil || got.Status != StatusRunning {
		t.Errorf("same member on another account should survive, got %v", got)
	}
	var sawReason bool
	for _, j := range recorded {
		if j.GetId() == dropped.ID && j.Error == "account sharing narrowed" {
			sawReason = true
		}
	}
	if !sawReason {
		t.Errorf("narrowing was not recorded with its reason")
	}
}
