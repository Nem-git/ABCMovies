package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/pacing"
	"github.com/nem-git/abcmovies/core/internal/policy"
	"github.com/nem-git/abcmovies/core/internal/store"
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
	if sess.Goal != delivery.GoalPlay || sess.Sink == nil {
		return nil, apiserver.ErrPlayMenuNotFound
	}
	staged := sess.StagedMenu()
	if len(staged) == 0 {
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
	for _, tr := range staged {
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

// classifyDeliveryRefusal attaches a typed admission reason to a refused
// start. The pacing governor's busy answer is the unhealthy-slot signal
// (PLAN.md §7.2): the provider is in backoff or the aggregate governor is at
// capacity. A plain transport failure is not a slot-health signal and stays
// an untyped failure (TECHNICAL-DECISIONS.md §1.44).
func classifyDeliveryRefusal(err error) (delivery.RefusalReason, bool) {
	var busy *pacing.BusyError
	if errors.As(err, &busy) {
		return delivery.RefusalUnhealthySlot, true
	}
	return delivery.RefusalUnspecified, false
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

// resolveAccountConstraints is the recorded-min(policy, provider_cap)
// grounding for one delivery request: a linked account reads its cap from
// the stored record (owner-declared at link time), a public operator
// account reads its from the slot config (PLAN.md §7.2). The cap stays a
// policy.ProviderCaps all the way to admission, so it can never be mistaken
// for the policy override returned alongside it. An account id pretty freely
// shaped by a provider lands in the history of a lot of config files, so
// the not-found branch admits with the instance policy alone rather than
// inventing a cap — the produce-sources call will honestly identify it.
func resolveAccountConstraints(ctx context.Context, cfg *config.Config, accts *accounts.Store, provider, accountID string) (policy.Set, policy.ProviderCaps, error) {
	if rec, err := accts.Get(ctx, accountID); err == nil {
		return nil, policy.CapsFor(rec.MaxConcurrentStreams), nil
	}
	for _, slot := range cfg.Slots.Providers {
		for _, acct := range slot.Accounts {
			if acct.ID != accountID {
				continue
			}
			override, err := policy.ParseOverlay(acct.Policy)
			if err != nil {
				return nil, policy.ProviderCaps{}, fmt.Errorf("policy: %v", err)
			}
			return override, policy.CapsFor(acct.MaxConcurrentStreams), nil
		}
	}
	return nil, policy.ProviderCaps{}, nil
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
	// Validated at config load; a broken value never reaches this point.
	timing, err := config.ParseDeliveryTiming(s.cfg.Delivery)
	if err != nil {
		return fmt.Errorf("delivery: timing already valid at load; this means memory corruption: %w", err)
	}
	accountStore := accounts.NewStore(s.stores.Vault, logger)
	eng := delivery.New(delivery.Options{
		SessionTTL:        timing.SessionTTL,
		HeartbeatInterval: timing.HeartbeatInterval,
		HeartbeatGrace:    timing.HeartbeatGrace,
		InstancePolicy:    instancePolicy,
		AccountConstraints: func(ctx context.Context, provider, accountID string) (policy.Set, policy.ProviderCaps, error) {
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
		// SessionDied runs the re-auth flow when a delivery discovers a
		// dead provider session (PLAN.md §7.5): the account flips to
		// EXPIRED, the owner is notified, and a re-auth job opens for the
		// owner. The flow is gated on the status transition, so a user
		// retrying play cannot spam the owner with duplicate events.
		SessionDied: func(accountID string) {
			reauthOnSessionDeath(context.Background(), accountStore, s.stores.Jobs, rt.Bus, accountID, logger)
		},
		// ClassifyRefusal attaches a typed admission reason to a refused
		// start. The pacing governor's busy answer is the unhealthy-slot
		// signal (PLAN.md §7.2): the provider is in backoff or the aggregate
		// governor is at capacity. A plain transport failure is not a
		// slot-health signal and stays an untyped failure — only pacing
		// admits the classification (TECHNICAL-DECISIONS.md §1.44).
		ClassifyRefusal: classifyDeliveryRefusal,
		Logger:          logger,
	})
	s.delivery = eng
	// The delivery watchdog is periodic work: Start launches it (PLAN.md
	// §10), never construction.
	// The liveness interval the engine enforces is the one clients must
	// heartbeat on — publish the same value, never a second copy of it.
	srv.SetHeartbeatInterval(timing.HeartbeatInterval)
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

// reauthOnSessionDeath runs the re-auth flow for an account whose provider
// session was discovered dead during a delivery (PLAN.md §7.5): the record
// flips to EXPIRED, the owner is notified, and a re-auth job opens in
// AWAITING_ACTION naming the owner as the only actor who may re-link. The
// flip gates the whole flow, so an account already awaiting re-auth is not
// re-announced when the user retries — the open job still waits for them.
func reauthOnSessionDeath(ctx context.Context, st *accounts.Store, jobs store.Store, bus *apiserver.InMemoryBus, accountID string, logger *slog.Logger) {
	rec, err := st.Get(ctx, accountID)
	if err != nil {
		// Nothing to expire: the account is gone or was never stored. A
		// store failure must not crash the delivery path that discovered
		// the death.
		logger.Error("re-auth: dead session for an unreadable account", "account", accountID, "err", err)
		return
	}
	if rec.Status == accounts.StatusExpired {
		return
	}
	if err := st.SetStatus(ctx, accountID, accounts.StatusExpired); err != nil {
		logger.Error("re-auth: failed to expire the account", "account", accountID, "err", err)
		return
	}
	bus.Publish(&corev1.EventEnvelope{
		Id:       fmt.Sprintf("evt-account-expired-%s-%d", accountID, time.Now().UnixNano()),
		Type:     corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_EXPIRED,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_OWNER,
		UserId:   rec.OwnerUserID,
		Payload: &corev1.EventEnvelope_AccountSession{
			AccountSession: &corev1.AccountSessionEvent{AccountId: accountID, Provider: rec.Provider},
		},
		EmittedAt: timestamppb.Now(),
	})
	// The job is the system of record for the open re-auth (PLAN.md §9.1).
	// Its deterministic id keeps one open job per account: a later death
	// after a successful re-link re-opens that same record.
	apiserver.RecordJobStatus(ctx, jobs, bus, &corev1.Job{
		Id:          "reauth-" + accountID,
		Kind:        corev1.JobKind_JOB_KIND_RE_AUTH,
		Status:      corev1.JobStatus_JOB_STATUS_AWAITING_ACTION,
		OwnerUserId: rec.OwnerUserID,
		AwaitingAction: &corev1.AwaitingAction{
			Kind:        &corev1.AwaitingAction_Common{Common: corev1.ActionType_ACTION_TYPE_RELINK},
			ActorUserId: rec.OwnerUserID,
		},
	})
}
