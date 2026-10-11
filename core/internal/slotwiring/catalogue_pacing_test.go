package slotwiring

import (
	"context"
	"testing"
	"time"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// The pace is the thing that protects the provider, so who gets to set it is
// the whole question: the operator's per-slot override wins, then the
// adapter's own handshake declaration, then the shipped default. A catalogue
// that declares nothing must land exactly on the shipped default, so
// declaring nothing stays behaviour-preserving.
func TestCataloguePacePrecedence(t *testing.T) {
	tests := []struct {
		name     string
		override string
		declared map[string]string
		want     time.Duration
	}{
		{
			name: "nothing declared falls back to the shipped default",
			want: defaultCataloguePace,
		},
		{
			name:     "adapter declaration beats the default",
			declared: map[string]string{cataloguePaceKey: "1s"},
			want:     time.Second,
		},
		{
			name:     "operator override beats the adapter declaration",
			override: "5s",
			declared: map[string]string{cataloguePaceKey: "1s"},
			want:     5 * time.Second,
		},
		{
			name:     "operator override wins even with no declaration",
			override: "250ms", want: 250 * time.Millisecond,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DeclaredPace(tc.override, tc.declared, cataloguePaceKey, defaultCataloguePace)
			if err != nil {
				t.Fatalf("DeclaredPace: %v", err)
			}
			if got != tc.want {
				t.Fatalf("pace = %v, want %v", got, tc.want)
			}
		})
	}
}

// A malformed pace is a broken adapter or a typo'd override — reported, not
// silently swallowed into "no pacing", which would leave the provider
// unprotected exactly when someone tried to tune it.
func TestCataloguePaceRejectsBadValues(t *testing.T) {
	for _, tc := range []struct {
		name     string
		override string
		declared map[string]string
	}{
		{name: "override not a duration", override: "fast"},
		{name: "override not positive", override: "0s"},
		{name: "declared not a duration", declared: map[string]string{cataloguePaceKey: "soon"}},
		{name: "declared not positive", declared: map[string]string{cataloguePaceKey: "-1s"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DeclaredPace(tc.override, tc.declared, cataloguePaceKey, defaultCataloguePace); err == nil {
				t.Fatal("want an error, got nil")
			}
		})
	}
}

// stubCatalogueClient counts calls; it never does I/O, because these tests
// assert spacing, not behaviour against a real provider.
type stubCatalogueClient struct{ calls int }

func (c *stubCatalogueClient) LookupTitle(context.Context, *slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
	c.calls++
	return &slotsv1.LookupTitleResponse{}, nil
}

func (c *stubCatalogueClient) GetMetadata(context.Context, *slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
	c.calls++
	return &slotsv1.GetMetadataResponse{}, nil
}

// The wrapper is the enforcement point: successive calls into one catalogue
// must be separated by that catalogue's pace, whichever method is used. The
// bound is deliberately loose (the assertion is "spaced, not simultaneous")
// so the test does not become a timing flake on a loaded machine.
func TestPacedCatalogueSpacesCalls(t *testing.T) {
	const pace = 120 * time.Millisecond
	inner := &stubCatalogueClient{}
	p := &pacedCatalogue{inner: inner, pace: pace}

	start := time.Now()
	ctx := context.Background()
	if _, err := p.LookupTitle(ctx, &slotsv1.LookupTitleRequest{}); err != nil {
		t.Fatalf("LookupTitle: %v", err)
	}
	if _, err := p.GetMetadata(ctx, &slotsv1.GetMetadataRequest{}); err != nil {
		t.Fatalf("GetMetadata: %v", err)
	}
	if _, err := p.LookupTitle(ctx, &slotsv1.LookupTitleRequest{}); err != nil {
		t.Fatalf("LookupTitle #2: %v", err)
	}
	elapsed := time.Since(start)

	if inner.calls != 3 {
		t.Fatalf("calls = %d, want 3", inner.calls)
	}
	if min := 2 * pace; elapsed < min {
		t.Fatalf("3 calls took %v, want at least %v (calls were not paced)", elapsed, min)
	}
	// The pace is an upper bound too: it spaces calls, it does not stall them.
	if max := 5 * pace; elapsed > max {
		t.Fatalf("3 calls took %v, want under %v (pacing is far slower than declared)", elapsed, max)
	}
}

// A zero pace would mean "no pacing at all", so it must never reach the
// wrapper: DeclaredPace substitutes the shipped default instead.
func TestDeclaredPaceZeroFallsBackToDefault(t *testing.T) {
	got, err := DeclaredPace("", nil, cataloguePaceKey, defaultCataloguePace)
	if err != nil {
		t.Fatalf("DeclaredPace: %v", err)
	}
	if got != defaultCataloguePace {
		t.Fatalf("pace = %v, want the shipped default %v", got, defaultCataloguePace)
	}
}
