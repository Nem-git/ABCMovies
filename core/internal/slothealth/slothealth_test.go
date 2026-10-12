package slothealth

import (
	"context"
	"testing"

	"github.com/nem-git/abcmovies/core/internal/store"
)

func TestProvider_RecordSuccessAndFailure(t *testing.T) {
	backend := store.NewInMemory()
	p, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if err := p.RecordProbeSuccess(ctx, "slot-1"); err != nil {
		t.Fatalf("RecordProbeSuccess: %v", err)
	}
	r, ok, err := p.Get(ctx, "slot-1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if !r.Healthy || r.FailureCount != 0 {
		t.Errorf("success record wrong: %+v", r)
	}
	if err := p.RecordProbeFailure(ctx, "slot-1", "AUTH", "bad"); err != nil {
		t.Fatalf("RecordProbeFailure: %v", err)
	}
	r, ok, err = p.Get(ctx, "slot-1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if r.Healthy || r.FailureCount != 1 || r.LastErrorKind != "AUTH" {
		t.Errorf("failure record wrong: %+v", r)
	}
	if err := p.RecordProbeFailure(ctx, "slot-1", "TIMEOUT", "timeout"); err != nil {
		t.Fatalf("RecordProbeFailure2: %v", err)
	}
	r, ok, _ = p.Get(ctx, "slot-1")
	if !ok || r.FailureCount != 2 {
		t.Errorf("failure count wrong: %+v", r)
	}
	list, err := p.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: len=%d err=%v", len(list), err)
	}
}

// TestRecordProbeTransitionHook pins the Batch-5 transition semantics: the
// hook fires exactly once per crossing of the healthy/unhealthy boundary —
// a failure on a healthy (or unseen) slot, a success on an unhealthy one —
// and never on repeats in the same state.
func TestRecordProbeTransitionHook(t *testing.T) {
	p, err := New(store.NewInMemory())
	if err != nil {
		t.Fatal(err)
	}
	var got []Transition
	p.SetTransitionHook(func(tr Transition) { got = append(got, tr) })
	ctx := context.Background()

	// First observation healthy: reachable-until-proven-otherwise, no news.
	if err := p.RecordProbeSuccess(ctx, "slot-1"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("healthy first observation fired %d transitions, want 0", len(got))
	}

	// Healthy -> unhealthy: one transition.
	if err := p.RecordProbeFailure(ctx, "slot-1", "NETWORK", "down"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("after failure: %d transitions, want 1", len(got))
	}
	if got[0].Healthy || !got[0].WasHealthy || got[0].SlotID != "slot-1" {
		t.Errorf("transition = %+v, want unhealthy flip of slot-1", got[0])
	}
	if got[0].LastErrorKind != "NETWORK" || got[0].LastErrorMessage != "down" {
		t.Errorf("transition error = %q/%q, want NETWORK/down", got[0].LastErrorKind, got[0].LastErrorMessage)
	}

	// Staying unhealthy: no spam.
	if err := p.RecordProbeFailure(ctx, "slot-1", "TIMEOUT", "still down"); err != nil {
		t.Fatal(err)
	}
	if err := p.RecordProbeFailure(ctx, "slot-1", "TIMEOUT", "still down"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("repeated failures fired %d transitions total, want 1", len(got))
	}

	// Unhealthy -> healthy: one recovery transition, no error detail.
	if err := p.RecordProbeSuccess(ctx, "slot-1"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("after recovery: %d transitions, want 2", len(got))
	}
	if !got[1].Healthy || got[1].WasHealthy {
		t.Errorf("transition = %+v, want healthy flip", got[1])
	}
	if got[1].LastErrorKind != "" || got[1].LastErrorMessage != "" {
		t.Errorf("recovery transition carries error %q/%q, want empty", got[1].LastErrorKind, got[1].LastErrorMessage)
	}
}

// TestRecordProbeFirstFailureIsATransition pins: a slot found down on its
// very first observation is news (one transition), unlike one found up.
func TestRecordProbeFirstFailureIsATransition(t *testing.T) {
	p, err := New(store.NewInMemory())
	if err != nil {
		t.Fatal(err)
	}
	var got []Transition
	p.SetTransitionHook(func(tr Transition) { got = append(got, tr) })

	if err := p.RecordProbeFailure(context.Background(), "slot-2", "AUTH", "bad creds"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("first failure fired %d transitions, want 1", len(got))
	}
	if got[0].Healthy || !got[0].WasHealthy {
		t.Errorf("transition = %+v, want unhealthy flip", got[0])
	}
}

// TestRecordProbeTransitionIsBestEffort pins: the probe record is durable
// even though nothing observes the hook, and wiring a hook late does not
// disturb stored state.
func TestRecordProbeTransitionIsBestEffort(t *testing.T) {
	backend := store.NewInMemory()
	p, err := New(backend)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := p.RecordProbeSuccess(ctx, "slot-3"); err != nil {
		t.Fatal(err)
	}
	if err := p.RecordProbeFailure(ctx, "slot-3", "NETWORK", "down"); err != nil {
		t.Fatal(err)
	}
	r, ok, err := p.Get(ctx, "slot-3")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if r.Healthy || r.FailureCount != 1 || r.LastErrorKind != "NETWORK" {
		t.Errorf("record = %+v, want unhealthy with failure recorded", r)
	}

	// A hook wired after the fact sees nothing retroactively.
	var got []Transition
	p.SetTransitionHook(func(tr Transition) { got = append(got, tr) })
	if len(got) != 0 {
		t.Fatalf("late-wired hook saw %d transitions, want 0", len(got))
	}
}
