// Package m6_test holds M6 acceptance tests over the policy surface: one
// enforcement point, whose rule is min(policy, provider_cap) per account, and
// one member-aggregate ceiling that does not let a member spread across
// accounts to double up (PLAN.md §7.2).
package m6_test

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/policy"
)

func muxSource() *corev1.MediaSource {
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_WHOLE_MUX,
		Tracks: []*corev1.Track{{
			Id:       "c1",
			Media:    &corev1.Track_Video{Video: &corev1.VideoTrack{Codec: "h264", Bitrate: 4_000_000}},
			Delivery: &corev1.TrackDelivery{Locations: []string{"https://example.test/v.m3u8"}},
		}},
	}
}

func newPolicyEngine(instance policy.Set, constraints func(ctx context.Context, provider, accountID string) (policy.Set, policy.Set, error), record func(*corev1.Job)) *delivery.Engine {
	e := delivery.New(delivery.Options{
		SessionTTL:         24 * time.Hour,
		InstancePolicy:     instance,
		AccountConstraints: constraints,
		SourceResolver:     staticResolver{src: muxSource()},
		RecordJob:          record,
	})
	return e
}

type staticResolver struct{ src *corev1.MediaSource }

func (r staticResolver) ProduceSources(context.Context, string, string, string) (*corev1.MediaSource, error) {
	return r.src, nil
}

func ctx() context.Context { return context.Background() }

func TestM6ProviderCapIsTheAccountCeiling(t *testing.T) {
	e := newPolicyEngine(
		policy.Set{"concurrentStreams": "4"},
		func(context.Context, string, string) (policy.Set, policy.Set, error) {
			return nil, policy.Set{"concurrentStreams": "2"}, nil
		},
		func(*corev1.Job) {},
	)
	defer e.Close()
	req := delivery.StartRequest{Goal: delivery.GoalDownload, MemberUserID: "u1", Provider: "jellyfin", AccountID: "acc1", Sink: "device"}
	if _, err := e.Start(ctx(), req); err != nil {
		t.Fatalf("1st: %v", err)
	}
	if _, err := e.Start(ctx(), req); err != nil {
		t.Fatalf("2nd: %v", err)
	}
	// Third through the same member+account must respect the declared cap (2),
	// even though the instance policy allows 4.
	if _, err := e.Start(ctx(), req); err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("third start: expected cap rejection, got %v", err)
	}
}

func TestM6MemberLimitSpansAccounts(t *testing.T) {
	e := newPolicyEngine(
		policy.Set{"concurrentStreams": "2"},
		func(context.Context, string, string) (policy.Set, policy.Set, error) {
			return nil, nil, nil // no caps; instance policy alone
		},
		func(*corev1.Job) {},
	)
	defer e.Close()
	r1 := delivery.StartRequest{Goal: delivery.GoalDownload, MemberUserID: "u1", Provider: "jellyfin", AccountID: "acc1", Sink: "device"}
	r2 := r1
	r2.AccountID = "acc2"
	if _, err := e.Start(ctx(), r1); err != nil {
		t.Fatalf("r1: %v", err)
	}
	if _, err := e.Start(ctx(), r2); err != nil {
		t.Fatalf("r2: %v", err)
	}
	// Same member on either account refuses a third; the account where the
	// two live must not absorb the third — this is the bypass that the old
	// per-(account,member) counter allowed.
	if _, err := e.Start(ctx(), r1); err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("third on acc1: expected member-aggregate cap, got %v", err)
	}
}

func TestM6JobRecordsEffectivePolicyAndCap(t *testing.T) {
	var recordCalls []*corev1.Job
	e := newPolicyEngine(
		policy.Set{"concurrentStreams": "4"},
		func(context.Context, string, string) (policy.Set, policy.Set, error) {
			return policy.Set{"concurrentStreams": "3"}, policy.Set{"concurrentStreams": "2"}, nil
		},
		func(j *corev1.Job) { recordCalls = append(recordCalls, j) },
	)
	defer e.Close()
	_, err := e.Start(ctx(), delivery.StartRequest{Goal: delivery.GoalDownload, MemberUserID: "u1", Provider: "jellyfin", AccountID: "acc1", Sink: "device"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(recordCalls) == 0 {
		t.Fatal("no job recorded")
	}
	job := recordCalls[len(recordCalls)-1]
	d := job.GetDelivery()
	if d == nil {
		t.Fatalf("job %q is not a delivery job", job.GetId())
	}
	p := d.GetPolicy().GetLimits()
	if p["concurrentStreams"] != "3" {
		t.Fatalf("recorded policy = %v, want concurrentStreams=3 (instance 4 overridden at account to 3)", p)
	}
	c := d.GetProviderCap().GetLimits()
	if c["concurrentStreams"] != "2" {
		t.Fatalf("recorded provider cap = %v, want 2", c)
	}
}
