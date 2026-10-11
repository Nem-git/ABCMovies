package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/pacing"
)

// The production classifier maps the pacing governor's busy answer — wherever
// it sits in the error chain — to the unhealthy-slot refusal, and leaves every
// other failure untyped (TECHNICAL-DECISIONS.md §1.44). A plain transport
// failure must never be dressed up as slot ill-health.
func TestClassifyDeliveryRefusal(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want delivery.RefusalReason
		ok   bool
	}{
		{
			name: "provider backoff",
			err:  &pacing.BusyError{Err: pacing.ErrDegraded},
			want: delivery.RefusalUnhealthySlot,
			ok:   true,
		},
		{
			name: "aggregate governor at capacity, wrapped",
			err:  fmt.Errorf("produce sources: %w", &pacing.BusyError{Err: pacing.ErrGovernorBusy}),
			want: delivery.RefusalUnhealthySlot,
			ok:   true,
		},
		{
			name: "transport failure stays untyped",
			err:  errors.New("provider unreachable"),
			want: delivery.RefusalUnspecified,
			ok:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := classifyDeliveryRefusal(tc.err)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("classifyDeliveryRefusal(%v) = (%q, %v), want (%q, %v)", tc.err, got, ok, tc.want, tc.ok)
			}
		})
	}
}
