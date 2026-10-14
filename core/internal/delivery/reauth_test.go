package delivery

import (
	"context"
	"errors"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// deadSessionResolver answers every resolve with one fixed error.
type deadSessionResolver struct{ err error }

func (r deadSessionResolver) ProduceSources(context.Context, string, string, string) (*corev1.MediaSource, error) {
	return nil, r.err
}

// noSessionMarker stands in for an adapter's typed no-live-session failure:
// structural, exactly like the real adapters' errors — the core recognizes
// one without importing any adapter.
type noSessionMarker struct{ msg string }

func (e noSessionMarker) Error() string   { return e.msg }
func (e noSessionMarker) NoSession() bool { return true }

func reauthStart() StartRequest {
	return StartRequest{
		Goal:         GoalPlay,
		Provider:     "jellyfin",
		AccountID:    "lnk_a",
		MemberUserID: "u1",
		Sink:         "relay",
		NativeID:     "movie-1",
	}
}

// A typed no-live-session failure discovered while starting a delivery must
// be reported to the composition root — the re-auth flow is the only
// recovery (PLAN.md §7.5) — while the start itself still fails.
func TestEngineStartReportsSessionDeath(t *testing.T) {
	var reported []string
	eng := New(Options{
		SourceResolver: deadSessionResolver{err: noSessionMarker{msg: "vault holds nothing"}},
		SessionDied:    func(accountID string) { reported = append(reported, accountID) },
	})
	if _, err := eng.Start(context.Background(), reauthStart()); err == nil {
		t.Fatal("starting a delivery on a dead session must fail")
	}
	if len(reported) != 1 || reported[0] != "lnk_a" {
		t.Fatalf("session-died reports = %v, want exactly [lnk_a]", reported)
	}
}

// Any other resolve failure stays the session's own business: without this
// gate an ordinary provider outage would flip every account EXPIRED.
func TestEngineOrdinaryResolveFailureStaysQuiet(t *testing.T) {
	var reported []string
	eng := New(Options{
		SourceResolver: deadSessionResolver{err: errors.New("provider unreachable")},
		SessionDied:    func(accountID string) { reported = append(reported, accountID) },
	})
	if _, err := eng.Start(context.Background(), reauthStart()); err == nil {
		t.Fatal("a failing resolve must fail the start")
	}
	if len(reported) != 0 {
		t.Fatalf("session-died reports = %v, want none for an ordinary failure", reported)
	}
}

// The hook is optional: a muted engine still fails the start cleanly.
func TestEngineStartWithoutSessionDiedHook(t *testing.T) {
	eng := New(Options{SourceResolver: deadSessionResolver{err: noSessionMarker{msg: "dead"}}})
	if _, err := eng.Start(context.Background(), reauthStart()); err == nil {
		t.Fatal("starting a delivery on a dead session must fail")
	}
}
