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

// managedDelivery is the delivery surface the API layer sees: the engine's
// Start/Heartbeat plus PlayMenu over the engine and the relay (PLAN.md §6.2).
// The engine is provider- and sink-agnostic, so this wrapper — which knows
// both the device sink's relay tokens and the session's plan — maps the
// staged menu to what GetPlayInfo returns. The provider is never touched
// again after Start: every later pull flows through the relay.
type managedDelivery struct {
	eng   *delivery.Engine
	relay *delivery.Relay
}

var _ apiserver.DeliveryManager = (*managedDelivery)(nil)

func (m managedDelivery) Start(ctx context.Context, req delivery.StartRequest) (*delivery.Session, error) {
	return m.eng.Start(ctx, req)
}

func (m managedDelivery) Heartbeat(id string) error { return m.eng.Heartbeat(id) }

// PlayMenu recovers a session's staged play menu, attaching each
// location-bearing track's relay token (PLAN.md §6.2). A download session,
// an unknown session, or a play session that never staged a menu (no sink,
// no tracks) is a clean miss — GetPlayInfo maps it to NotFound.
func (m managedDelivery) PlayMenu(sessionID string) (*apiserver.PlayMenu, error) {
	sess, ok := m.eng.Get(sessionID)
	if !ok {
		return nil, apiserver.ErrPlayMenuNotFound
	}
	if sess.Goal != delivery.GoalPlay || sess.Sink == nil || len(sess.Menu) == 0 {
		return nil, apiserver.ErrPlayMenuNotFound
	}
	device, ok := sess.Sink.(*delivery.DeviceSink)
	if !ok {
		return nil, apiserver.ErrPlayMenuNotFound
	}
	menu := &apiserver.PlayMenu{
		SessionID:    sess.ID,
		MemberUserID: sess.Context.GetMemberUserId(),
		Container:    planContainer(sess.Plan),
	}
	for _, tr := range sess.Menu {
		token, ok := device.RelayToken(tr.GetId())
		if !ok {
			// A carried-in track has no location and thus no relay ticket; the
			// player reads it off the carrier track's delivery (WHOLE_MUX),
			// so it is skipped here, not surfaced with an empty URL.
			continue
		}
		menu.Tracks = append(menu.Tracks, apiserver.PlayMenuTrack{TrackID: tr.GetId(), Track: tr, RelayToken: token})
	}
	return menu, nil
}

// planContainer extracts the deliverable's container from a play plan: the
// remux step that names it, or "" for passthrough (nothing is known yet).
func planContainer(p delivery.Plan) string {
	for i := range p.Steps {
		if p.Steps[i].Kind == delivery.StepRemux && p.Steps[i].Params.Remux != nil {
			return p.Steps[i].Params.Remux.Container
		}
	}
	return ""
}

// armDelivery builds the delivery engine over the composed resolver and sink
// factory, starts its watchdog, and hands it — wrapped as managedDelivery — to
// the API service so delivery RPCs start working. The connected-account
// surface (library, credential probers) is armed at the same time. It uses a
// background context so the watchdog lives for the stack's lifetime;
// Stack.Close stops it.
func (s *Stack) armDelivery(rt *SlotRuntime, logger *slog.Logger) error {
	srv, ok := s.service.(*apiserver.Server)
	if !ok {
		return nil
	}
	eng := delivery.New(delivery.Options{
		SessionTTL:        24 * time.Hour,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatGrace:    90 * time.Second,
		ConcurrentStreams: 3,
		SourceResolver:    compositeResolver{resolvers: rt.Resolvers},
		SinkFactory:       rt.Sinks,
		RecordJob:         func(j *corev1.Job) { s.persistDeliveryJob(rt, j) },
		// MenuReady announces a staged play menu once, at Start (PLAN.md
		// §6.2): a subscriber that misses the notification recovers by
		// GetPlayInfo, per the bus's at-most-once contract (§9.2).
		MenuReady: func(sess *delivery.Session) {
			rt.Bus.Publish(&corev1.EventEnvelope{
				Id:       fmt.Sprintf("evt-menu-%s", sess.ID),
				Type:     corev1.EventType_EVENT_TYPE_DELIVERY_PLAY_MENU_READY,
				Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
				UserId:   sess.Context.GetMemberUserId(),
				Payload: &corev1.EventEnvelope_PlayMenuReady{
					PlayMenuReady: &corev1.PlayMenuReadyEvent{JobId: sess.ID},
				},
				EmittedAt: timestamppb.Now(),
			})
		},
		Logger: logger,
	})
	s.delivery = eng
	go eng.Watch(context.Background())
	srv.SetDelivery(managedDelivery{eng: eng, relay: rt.Relay})
	srv.SetLibrary(rt.Library)
	for provider, prober := range rt.Probers {
		srv.SetProber(provider, prober)
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
