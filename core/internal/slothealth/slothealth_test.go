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
