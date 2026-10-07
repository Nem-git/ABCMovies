package app

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/policy"
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

func (m managedDelivery) Heartbeat(id string, memberUserID string) error {
	return m.eng.Heartbeat(id, memberUserID)
}

func (m managedDelivery) RevokeAllOnAccount(accountID string) int {
	return m.eng.RevokeAllOnAccount(accountID)
}

func (m managedDelivery) RevokeOthersOnAccount(provider, accountID string, keepMembers []string) int {
	return m.eng.RevokeOthersOnAccount(provider, accountID, keepMembers)
}

func (m managedDelivery) ApplyAccountCap(ctx context.Context, provider, accountID string, enforceNow bool) (int, error) {
	return m.eng.ApplyAccountCap(ctx, provider, accountID, enforceNow)
}

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

// declaredCap renders an account's persisted/declared concurrency ceiling
// as a provider-cap policy set; a zero cap stays unbound (nil), since the
// declaredCap renders an account's persisted or declared concurrency
// ceiling as a provider-cap policy set; a zero cap stays unbound (nil),
// since the instance policy still applies.
func declaredCap(n uint32) policy.Set {
	if n == 0 {
		return nil
	}
	return policy.Set{policy.KeyConcurrentStreams: strconv.Itoa(int(n))}
}

// resolveAccountConstraints is the recorded-min(policy, provider_cap)
// grounding for one delivery request: a linked account reads its cap from
// the stored record (owner-declared at link time), a public operator
// account reads its from the slot config. An account id pretty freely
// shaped by a provider lands in the history of a lot of config files, so
// the not-found branch admits with the instance policy alone rather than
// inventing a cap — the produce-sources call will honestly identify it.
func resolveAccountConstraints(ctx context.Context, cfg *config.Config, accts *accounts.Store, provider, accountID string) (policy.Set, policy.Set, error) {
	if rec, err := accts.Get(ctx, accountID); err == nil {
		return nil, declaredCap(rec.MaxConcurrentStreams), nil
	}
	for _, slot := range cfg.Slots.Providers {
		for _, acct := range slot.Accounts {
			if acct.ID != accountID {
				continue
			}
			override, err := policy.ParseOverlay(acct.Policy)
			if err != nil {
				return nil, nil, fmt.Errorf("policy: %v", err)
			}
			return override, declaredCap(acct.MaxConcurrentStreams), nil
		}
	}
	return nil, nil, nil
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
	instancePolicy, err := policy.ParseInstance(s.cfg.Policy)
	if err != nil {
		return fmt.Errorf("delivery: policy already valid at load; this means memory corruption: %w", err)
	}
	accountStore := accounts.NewStore(s.stores.Vault, logger)
	eng := delivery.New(delivery.Options{
		SessionTTL:        24 * time.Hour,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatGrace:    90 * time.Second,
		InstancePolicy:    instancePolicy,
		AccountConstraints: func(ctx context.Context, provider, accountID string) (policy.Set, policy.Set, error) {
			return resolveAccountConstraints(ctx, s.cfg, accountStore, provider, accountID)
		},
		SourceResolver: compositeResolver{resolvers: rt.Resolvers},
		SinkFactory:    rt.Sinks,
		// The engine is the one announcer of delivery-job status: it
		// observes every transition, including the ones no client
		// requested (expiry, revocation, cleanup), and the handler keeps
		// only its store write for GetJob freshness. Both call the same
		// RecordJobStatus the harnesses use, so fixtures exercise the
		// production path rather than a re-implementation.
		RecordJob: func(j *corev1.Job) {
			apiserver.RecordJobStatus(context.Background(), s.stores.Jobs, rt.Bus, j)
		},
		// MenuReady announces a staged play menu once, at Start (PLAN.md
		// §6.2). The slot runtime bus and the API bus are one object in the
		// composed stack, so a single publish covers both audiences; a
		// subscriber that misses it recovers by GetPlayInfo, per the bus's
		// at-most-once contract (§9.2).
		MenuReady: func(sess *delivery.Session) {
			env := &corev1.EventEnvelope{
				Id:       fmt.Sprintf("evt-menu-%s", sess.ID),
				Type:     corev1.EventType_EVENT_TYPE_DELIVERY_PLAY_MENU_READY,
				Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
				UserId:   sess.Context.GetMemberUserId(),
				Payload: &corev1.EventEnvelope_PlayMenuReady{
					PlayMenuReady: &corev1.PlayMenuReadyEvent{JobId: sess.ID},
				},
				EmittedAt: timestamppb.Now(),
			}
			rt.Bus.Publish(env)
		},
		Logger: logger,
	})
	s.delivery = eng
	go eng.Watch(context.Background())
	srv.SetDelivery(managedDelivery{eng: eng, relay: rt.Relay})
	srv.SetLibrary(rt.Library)
	srv.SetLiveSearcher(liveSlotsFromBuilt(rt.Providers, rt.Library, logger))
	for provider, prober := range rt.Probers {
		srv.SetProber(provider, prober)
	}
	// Arm the runtime link path both ways: a freshly-linked account routes to
	// its provider slot and comes alive without a restart, and an unlinked one
	// leaves the running slot on the way out (PLAN.md §5.1).
	srv.SetAttacher(rt)
	srv.SetDropper(rt)
	return nil
}
