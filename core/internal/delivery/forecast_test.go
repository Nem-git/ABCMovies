package delivery

import (
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

func TestForegroundGateDefersWhileAccountBusy(t *testing.T) {
	e, res, _ := newTestEngine(Options{
		SessionTTL: time.Hour,
		RecordJob:  func(*corev1.Job) {},
	})
	res.source = wholeMuxSource()
	defer e.Close()
	g := NewForegroundGate()
	g.Bind(e)

	// No session: admitted.
	release, allow := g.Admit("jellyfin", "a1")
	if !allow || release == nil {
		t.Fatalf("empty gate should admit any round, got allowed=%v", allow)
	}
	release()

	// A live session through this account: background rounds defer.
	req := StartRequest{Goal: GoalDownload, MemberUserID: "u1", Provider: "jellyfin", AccountID: "a1", Sink: "disk"}
	if _, err := e.Start(t.Context(), req); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, allow := g.Admit("jellyfin", "a1"); allow {
		t.Fatal("round must yield while an active session uses the account")
	}

	// A different account is untouched.
	release2, allow2 := g.Admit("jellyfin", "a2")
	if !allow2 || release2 == nil {
		t.Fatal("round on another account must admit")
	}
	release2()

	// Completing the foreground session releases the seat.
	for _, s := range e.sessions {
		if s.Status == StatusRunning {
			if err := e.Complete(s.ID); err != nil {
				t.Fatalf("complete: %v", err)
			}
		}
	}
	release3, allow3 := g.Admit("jellyfin", "a1")
	if !allow3 || release3 == nil {
		t.Fatal("round must admit again once the account is idle")
	}
	release3()
}

func TestForegroundGateSingleflightsRounds(t *testing.T) {
	g := NewForegroundGate()
	g.Bind(&Engine{}) // no sessions; engine counts are zero

	release, allow := g.Admit("jellyfin", "a1")
	if !allow {
		t.Fatal("first round must admit")
	}
	if _, allow := g.Admit("jellyfin", "a1"); allow {
		t.Fatal("overlapping round on the same account must defer")
	}
	release()
	release, allow = g.Admit("jellyfin", "a1")
	if !allow || release == nil {
		t.Fatal("seat must free after release")
	}
	release()
}
