package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/policy"
)

// classifierMarker stands in for a cross-package "unhealthy" signal the
// composition root knows how to name — the pacing governor's busy answer in
// production. The engine never imports it; the classifier handed to Options
// does.
type classifierMarker struct{}

func (classifierMarker) Error() string { return "provider degraded" }

// A concurrent-stream refusal is a typed AT_CAPACITY refusal carried at
// ResourceExhausted — not merely a message a caller must string-match.
func TestStartRefusalAtCapacityIsTyped(t *testing.T) {
	now := time.Now()
	e, res, _ := newTestEngine(Options{
		SessionTTL:     config.DefaultSessionTTL,
		InstancePolicy: policy.Set{policy.KeyConcurrentStreams: "1"},
		Now:            func() time.Time { return now },
		RecordJob:      func(*corev1.Job) {},
	})
	res.source = wholeMuxSource()
	defer e.Close()

	// A download fills the single seat; a play at the cap refuses loudly
	// (PLAN.md §6.5: play gets a priority path — start immediately or not at
	// all).
	if _, err := e.Start(context.Background(), StartRequest{
		Goal:         GoalDownload,
		MemberUserID: "u1",
		Provider:     "jellyfin",
		AccountID:    "acc1",
		NativeID:     "movie-1",
		Sink:         "relay",
	}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	_, err := e.Start(context.Background(), StartRequest{
		Goal:         GoalPlay,
		MemberUserID: "u1",
		Provider:     "jellyfin",
		AccountID:    "acc1",
		NativeID:     "movie-1",
		Sink:         "relay",
	})
	if err == nil {
		t.Fatal("second start should hit the cap of 1")
	}
	if got := Code(err); got != int32(codes.ResourceExhausted) {
		t.Errorf("code = %v, want ResourceExhausted", got)
	}
	if got := Reason(err); got != RefusalAtCapacity {
		t.Errorf("reason = %q, want %q", got, RefusalAtCapacity)
	}
	if !strings.Contains(err.Error(), "concurrent-stream cap") {
		t.Errorf("message = %q, want it to name the cap", err.Error())
	}
}

// A dead provider session is refused as NO_SESSION/FailedPrecondition — the
// caller is told to re-link — while the composition root is still told the
// session died, so the re-auth flow runs (PLAN.md §7.5).
func TestStartRefusalNoSessionIsTyped(t *testing.T) {
	var reported []string
	e, res, _ := newTestEngine(Options{
		SessionTTL:  config.DefaultSessionTTL,
		RecordJob:   func(*corev1.Job) {},
		SessionDied: func(accountID string) { reported = append(reported, accountID) },
	})
	res.err = noSessionMarker{msg: "vault holds nothing"}
	defer e.Close()

	_, err := e.Start(context.Background(), reauthStart())
	if err == nil {
		t.Fatal("a dead session must fail the start")
	}
	if got := Code(err); got != int32(codes.FailedPrecondition) {
		t.Errorf("code = %v, want FailedPrecondition", got)
	}
	if got := Reason(err); got != RefusalNoSession {
		t.Errorf("reason = %q, want %q", got, RefusalNoSession)
	}
	if !strings.Contains(err.Error(), "re-link") {
		t.Errorf("message = %q, want it to tell the caller to re-link", err.Error())
	}
	if len(reported) != 1 || reported[0] != "lnk_a" {
		t.Fatalf("session-died reports = %v, want exactly [lnk_a]", reported)
	}
}

// The composition root's classifier turns a cross-package signal into a typed
// UNHEALTHY_SLOT refusal carried at Unavailable.
func TestStartRefusalClassifierMapsUnhealthySlot(t *testing.T) {
	e, res, _ := newTestEngine(Options{
		SessionTTL: config.DefaultSessionTTL,
		RecordJob:  func(*corev1.Job) {},
		ClassifyRefusal: func(err error) (RefusalReason, bool) {
			var m classifierMarker
			if errors.As(err, &m) {
				return RefusalUnhealthySlot, true
			}
			return RefusalUnspecified, false
		},
	})
	res.err = fmt.Errorf("produce sources: %w", classifierMarker{})
	defer e.Close()

	_, err := e.Start(context.Background(), reauthStart())
	if err == nil {
		t.Fatal("an unhealthy slot must fail the start")
	}
	if got := Code(err); got != int32(codes.Unavailable) {
		t.Errorf("code = %v, want Unavailable", got)
	}
	if got := Reason(err); got != RefusalUnhealthySlot {
		t.Errorf("reason = %q, want %q", got, RefusalUnhealthySlot)
	}
	if !strings.Contains(err.Error(), "jellyfin") {
		t.Errorf("message = %q, want it to name the provider", err.Error())
	}
}

// A classifier that does not claim a failure leaves it a plain error — Unknown
// code, no reason, and the original text preserved through the wrap. This is
// the guard against inventing a refusal reason for every provider hiccup.
func TestStartRefusalUnclassifiedStaysPlain(t *testing.T) {
	e, res, _ := newTestEngine(Options{
		SessionTTL: config.DefaultSessionTTL,
		RecordJob:  func(*corev1.Job) {},
		ClassifyRefusal: func(err error) (RefusalReason, bool) {
			return RefusalUnspecified, false
		},
	})
	res.err = errors.New("provider unreachable")
	defer e.Close()

	_, err := e.Start(context.Background(), reauthStart())
	if err == nil {
		t.Fatal("a failing resolve must fail the start")
	}
	if got := Code(err); got != int32(codes.Unknown) {
		t.Errorf("code = %v, want Unknown", got)
	}
	if got := Reason(err); got != RefusalUnspecified {
		t.Errorf("reason = %q, want none", got)
	}
	if !strings.Contains(err.Error(), "provider unreachable") {
		t.Errorf("message = %q, want the original failure preserved", err.Error())
	}
}

// A dead session is classified NO_SESSION even when the classifier would also
// claim the error: the caller's required action (re-link) must never be masked
// by a generic slot-health reason, and the classifier must not even run.
func TestStartRefusalNoSessionBeatsClassifier(t *testing.T) {
	claimed := false
	e, res, _ := newTestEngine(Options{
		SessionTTL: config.DefaultSessionTTL,
		RecordJob:  func(*corev1.Job) {},
		ClassifyRefusal: func(err error) (RefusalReason, bool) {
			claimed = true
			return RefusalUnhealthySlot, true
		},
	})
	res.err = noSessionMarker{msg: "dead"}
	defer e.Close()

	_, err := e.Start(context.Background(), reauthStart())
	if err == nil {
		t.Fatal("a dead session must fail the start")
	}
	if got := Reason(err); got != RefusalNoSession {
		t.Errorf("reason = %q, want %q", got, RefusalNoSession)
	}
	if claimed {
		t.Error("classifier ran for a no-session failure; NO_SESSION must win")
	}
}
