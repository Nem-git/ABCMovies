package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// switchingResolver answers for any provider it is asked about and records
// every provider it was asked to resolve. It stands in for a stack where a
// title is reachable through more than one provider: if the engine ever failed
// over, it would call ProduceSources a second time with the other provider id
// and succeed — so a start that quietly succeeds is a start that switched.
type switchingResolver struct {
	primary string
	calls   []string
}

func (r *switchingResolver) ProduceSources(_ context.Context, provider, _ string, _ string) (*corev1.MediaSource, error) {
	r.calls = append(r.calls, provider)
	if provider == r.primary {
		return nil, errors.New("provider unreachable")
	}
	return staticSource(), nil
}

// Failover is the user's choice, not the engine's (PLAN.md §6.5). When the
// provider the caller named cannot resolve, the start fails: the engine never
// reaches for another provider that could serve the same title, never
// announces a menu, and never records a job for a switch that did not happen.
// This is the negative lock that keeps "no automatic switching" true.
func TestStartDoesNotFailOverToAnotherProvider(t *testing.T) {
	res := &switchingResolver{primary: "jellyfin"}
	var menus, jobs int
	e := New(Options{
		SessionTTL:     24 * time.Hour,
		SourceResolver: res,
		SinkFactory:    &fakeSinkFactory{},
		MenuReady:      func(*Session) { menus++ },
		RecordJob:      func(*corev1.Job) { jobs++ },
	})
	defer e.Close()

	_, err := e.Start(context.Background(), StartRequest{
		Goal:         GoalPlay,
		MemberUserID: "u1",
		Provider:     "jellyfin",
		AccountID:    "acc1",
		NativeID:     "movie-1",
		Sink:         "device",
	})
	if err == nil {
		t.Fatal("a failing provider must fail the start, not switch to another")
	}
	if len(res.calls) != 1 || res.calls[0] != "jellyfin" {
		t.Fatalf("resolver calls = %v, want exactly [jellyfin]", res.calls)
	}
	if menus != 0 {
		t.Errorf("menus announced = %d, want 0", menus)
	}
	if jobs != 0 {
		t.Errorf("jobs recorded = %d, want 0", jobs)
	}
	if got := e.LiveSessions(); got != 0 {
		t.Errorf("live sessions = %d, want 0", got)
	}
}
