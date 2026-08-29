package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/delivery"
)

// compositeResolver routes produce-sources to the provider adapter wired for
// the requested provider (a slot instance id, TECHNICAL-DECISIONS.md §1.25).
type compositeResolver struct {
	resolvers map[string]delivery.Resolver
}

func (c compositeResolver) ProduceSources(ctx context.Context, provider, accountID, nativeID string) (*corev1.MediaSource, error) {
	r, ok := c.resolvers[provider]
	if !ok {
		return nil, fmt.Errorf("no resolver wired for provider %q", provider)
	}
	return r.ProduceSources(ctx, provider, accountID, nativeID)
}

// armDelivery builds the delivery engine over the composed resolver and sink
// factory, starts its watchdog, and hands it to the API service so delivery
// RPCs start working. It uses a background context so the watchdog lives for
// the stack's lifetime; Stack.Close stops it.
func (s *Stack) armDelivery(rt *SlotRuntime, logger *slog.Logger) error {
	eng := delivery.New(delivery.Options{
		SessionTTL:        24 * time.Hour,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatGrace:    90 * time.Second,
		ConcurrentStreams: 3,
		SourceResolver:    compositeResolver{resolvers: rt.Resolvers},
		SinkFactory:       rt.Sinks,
		RecordJob:         func(j *corev1.Job) { s.persistDeliveryJob(rt, j) },
		Logger:            logger,
	})
	s.delivery = eng
	go eng.Watch(context.Background())
	if srv, ok := s.service.(*apiserver.Server); ok {
		srv.SetDelivery(eng)
	}
	return nil
}

// persistDeliveryJob writes a delivery session's system-of-record Job and
// announces its status event, mirroring the API service's job persistence
// (PLAN.md §9.1, §9.2) so GetJob and Subscribe stay current.
func (s *Stack) persistDeliveryJob(rt *SlotRuntime, job *corev1.Job) {
	if job == nil {
		return
	}
	raw, err := proto.Marshal(job)
	if err != nil {
		return
	}
	_ = s.stores.Jobs.Put(context.Background(), "job:"+job.GetId(), raw)
	rt.Bus.Publish(&corev1.EventEnvelope{
		Id:       fmt.Sprintf("evt-delivery-%s", job.GetId()),
		Type:     corev1.EventType_EVENT_TYPE_JOB_STATUS,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
		UserId:   job.GetOwnerUserId(),
		Payload: &corev1.EventEnvelope_JobStatus{
			JobStatus: &corev1.JobStatusEvent{JobId: job.GetId(), Status: job.GetStatus()},
		},
		EmittedAt: timestamppb.Now(),
	})
}
