package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/pacing"
	"github.com/nem-git/abcmovies/core/internal/policy"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

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

func (m managedDelivery) Cancel(jobID string) (*delivery.Session, error) {
	return m.eng.Cancel(jobID)
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
	relay, ok := sess.Sink.(*delivery.RelaySink)
	if !ok {
		return nil, apiserver.ErrPlayMenuNotFound
	}

	format := sess.Plan.Packages()
	if format != delivery.PackagingNone {
		// Packaged: the menu is a manifest the Package step granted. The
		// manifest references the same per-track relay tokens the raw branch
		// would list, so there is no second granting path.
		token, _ := m.eng.ManifestToken(sess.ID)
		return &apiserver.PlayMenu{
			SessionID:     sess.ID,
			MemberUserID:  sess.Context.GetMemberUserId(),
			Container:     planContainer(sess.Plan),
			Format:        format,
			ManifestToken: token,
		}, nil
	}

	menu := &apiserver.PlayMenu{
		SessionID:    sess.ID,
		MemberUserID: sess.Context.GetMemberUserId(),
		Container:    planContainer(sess.Plan),
	}
	for _, tr := range staged {
		token, ok := relay.RelayToken(tr.GetId())
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
func resolveAccountConstraints(ctx context.Context, slots []*corev1.Slot, accts *accounts.Store, provider, accountID string) (policy.Set, policy.ProviderCaps, error) {
	if rec, err := accts.Get(ctx, accountID); err == nil {
		return nil, policy.CapsFor(rec.MaxConcurrentStreams), nil
	}
	for _, slot := range slots {
		if slot.GetKind() != corev1.SlotKind_SLOT_KIND_PROVIDER {
			continue
		}
		for _, acct := range slot.GetProvider().GetAccounts() {
			if acct.GetId() != accountID {
				continue
			}
			override, err := policy.ParseOverlay(acct.GetPolicy())
			if err != nil {
				return nil, policy.ProviderCaps{}, fmt.Errorf("policy: %v", err)
			}
			return override, policy.CapsFor(acct.GetMaxConcurrentStreams()), nil
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
	// The DRM client is built from the store's DRM slots. If no DRM slot is
	// configured, it is nil and DRM content is refused at resolve.
	var drmSlots []*corev1.Slot
	drmSlotsList, err := s.slotStore.ListByKind(context.Background(), corev1.SlotKind_SLOT_KIND_DRM, "")
	if err != nil {
		return fmt.Errorf("delivery: list drm slots: %w", err)
	}
	drmSlots = drmSlotsList
	drmClient := slotwiring.SetupDRM(context.Background(), drmSlots, logger)
	// The opener produces clear bytes for both pull (relay) and push (disk)
	// sinks. With a DRM client wired, the opener can acquire keys and
	// decrypt DRM-encrypted content; without one, it is a passthrough.
	opener := delivery.NewOpener(drmClient, nil, logger)
	eng := delivery.New(delivery.Options{
		SessionTTL:        timing.SessionTTL,
		HeartbeatInterval: timing.HeartbeatInterval,
		HeartbeatGrace:    timing.HeartbeatGrace,
		InstancePolicy:    instancePolicy,
		Relay:             s.relay,
		Opener:            opener,
		DRMClient:         drmClient,
		AccountConstraints: func(ctx context.Context, provider, accountID string) (policy.Set, policy.ProviderCaps, error) {
			slots, err := s.slotStore.ListByKind(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "")
			if err != nil {
				return nil, policy.ProviderCaps{}, fmt.Errorf("slots: %w", err)
			}
			return resolveAccountConstraints(ctx, slots, accountStore, provider, accountID)
		},
		// The engine is built once and lives as long as the stack; these
		// seams read the runtime that is live at call time, so a slot reload
		// swaps the resolvers and sinks the engine uses without rebuilding it
		// (D8).
		SourceResolver: stackResolver{s: s},
		SinkFactory:    stackSinkFactory{s: s},
		// The engine is the one announcer of delivery-job status: it
		// observes every transition, including the ones no client
		// requested (expiry, revocation, cleanup), and the handler keeps
		// only its store write for GetJob freshness. Both call the same
		// RecordJobStatus the harnesses use, so fixtures exercise the
		// production path rather than a re-implementation.
		RecordJob: func(j *corev1.Job) {
			apiserver.RecordJobStatus(context.Background(), s.stores.Jobs, s.bus, j)
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
			s.bus.Publish(env)
		},
		// SessionDied runs the re-auth flow when a delivery discovers a
		// dead provider session (PLAN.md §7.5): the account flips to
		// EXPIRED, the owner is notified, and a re-auth job opens for the
		// owner. The flow is gated on the status transition, so a user
		// retrying play cannot spam the owner with duplicate events.
		SessionDied: func(accountID string) {
			reauthOnSessionDeath(context.Background(), accountStore, s.stores.Jobs, s.bus, accountID, logger)
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
	// Fail orphaned queued jobs: a queued download is persisted in the store,
	// but the in-memory queue is empty after a restart, so any QUEUED job in
	// the store has no live session behind it. Mark it FAILED with an honest
	// reason — never a phantom (PLAN.md §9.1).
	failOrphanedQueued(context.Background(), s.stores.Jobs, s.bus, logger)
	// The delivery watchdog is periodic work: Start launches it (PLAN.md
	// §10), never construction.
	// The liveness interval the engine enforces is the one clients must
	// heartbeat on — publish the same value, never a second copy of it.
	srv.SetHeartbeatInterval(timing.HeartbeatInterval)
	srv.SetDelivery(managedDelivery{eng: eng, relay: rt.Relay})
	// Everything below reads the runtime that is live at call time, so a live
	// slot reload takes effect on the next call without re-arming the server.
	// Probers are the exception: their set is replaced wholesale by the reload
	// under the server's own lock.
	srv.SetLibrary(stackLibrary{s: s})
	srv.SetLiveSearcher(stackLiveSearch{s: s, log: logger})
	srv.SetAttacher(stackAttacher{s: s})
	srv.SetDropper(stackDropper{s: s})
	srv.SetProbers(rt.Probers)
	return nil
}

// stackResolver routes produce-sources through whatever runtime is live at
// call time, so a slot reload changes which adapters exist without rebuilding
// the delivery engine (D8).
type stackResolver struct{ s *Stack }

func (r stackResolver) ProduceSources(ctx context.Context, provider, accountID, nativeID string) (*corev1.MediaSource, error) {
	rt := r.s.currentSlots()
	if rt == nil {
		return nil, fmt.Errorf("no resolver wired for provider %q", provider)
	}
	res, ok := rt.Resolvers[provider]
	if !ok {
		return nil, fmt.Errorf("no resolver wired for provider %q", provider)
	}
	return res.ProduceSources(ctx, provider, accountID, nativeID)
}

// stackSinkFactory builds a delivery sink from whatever runtime is live at
// call time, so a reload that adds, removes, or replaces a sink slot applies
// to the next session without rebuilding the engine (D8).
type stackSinkFactory struct{ s *Stack }

func (f stackSinkFactory) NewSink(ctx context.Context, s *delivery.Session, tracks []*corev1.Track) (delivery.Sink, error) {
	rt := f.s.currentSlots()
	if rt == nil || rt.Sinks == nil {
		return nil, fmt.Errorf("no sink configured")
	}
	return rt.Sinks.NewSink(ctx, s, tracks)
}

// stackAttacher routes a runtime-linked account into its live provider slot,
// reading the current runtime at call time.
type stackAttacher struct{ s *Stack }

func (a stackAttacher) AttachAccount(rec accounts.Record) error {
	rt := a.s.currentSlots()
	if rt == nil {
		return fmt.Errorf("no live slot runtime to attach account %q", rec.ID)
	}
	return rt.AttachAccount(rec)
}

// stackDropper takes an unlinked account out of its live provider slot,
// reading the current runtime at call time.
type stackDropper struct{ s *Stack }

func (d stackDropper) DropAccount(rec accounts.Record) error {
	rt := d.s.currentSlots()
	if rt == nil {
		return nil
	}
	return rt.DropAccount(rec)
}

// stackLibrary is the LibrarySeam over the live runtime: every library and
// account RPC reads the library service composed by the runtime that is live
// when the call arrives, so a slot reload changes the derived libraries
// without re-arming the server (D8).
type stackLibrary struct{ s *Stack }

func (l stackLibrary) Library(ctx context.Context, userID string) ([]*corev1.LibraryEntry, error) {
	lib, err := l.live()
	if err != nil {
		return nil, err
	}
	return lib.Library(ctx, userID)
}

func (l stackLibrary) Metadata(ctx context.Context, ref string) (*corev1.TitleMetadata, bool, error) {
	lib, err := l.live()
	if err != nil {
		return nil, false, err
	}
	return lib.Metadata(ctx, ref)
}

func (l stackLibrary) ReachAuthorized(accountID, userID string) (library.Reach, bool) {
	lib, err := l.live()
	if err != nil {
		return library.Reach{}, false
	}
	return lib.ReachAuthorized(accountID, userID)
}

func (l stackLibrary) ReachesForUser(userID string) []library.Reach {
	lib, err := l.live()
	if err != nil {
		return nil
	}
	return lib.ReachesForUser(userID)
}

func (l stackLibrary) SetReachSharing(accountID string, visibility accounts.Visibility, members []string) error {
	lib, err := l.live()
	if err != nil {
		return err
	}
	return lib.SetReachSharing(accountID, visibility, members)
}

func (l stackLibrary) RemoveReach(accountID string) {
	if lib, err := l.live(); err == nil {
		lib.RemoveReach(accountID)
	}
}

func (l stackLibrary) RefreshAvailability(ctx context.Context, accountID string, nativeIDs []string) (sourcecache.Stats, error) {
	lib, err := l.live()
	if err != nil {
		return sourcecache.Stats{}, err
	}
	return lib.RefreshAvailability(ctx, accountID, nativeIDs)
}

func (l stackLibrary) live() (*library.Service, error) {
	rt := l.s.currentSlots()
	if rt == nil || rt.Library == nil {
		return nil, fmt.Errorf("library engine not configured")
	}
	return rt.Library, nil
}

// stackLiveSearch runs the explicit lazy-provider refresh against the provider
// slots of whatever runtime is live at call time.
type stackLiveSearch struct {
	s   *Stack
	log *slog.Logger
}

func (l stackLiveSearch) Run(ctx context.Context, req *apiv1.LiveSearchRequest) (*apiv1.LiveSearchResponse, error) {
	rt := l.s.currentSlots()
	if rt == nil {
		return &apiv1.LiveSearchResponse{}, nil
	}
	return liveSlotsFromBuilt(rt.Providers, rt.Library, l.log).Run(ctx, req)
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

// failOrphanedQueued marks queued jobs with no live session as FAILED. It is
// called at startup: a queued download is persisted in the store, but the
// in-memory queue is empty after a restart, so any QUEUED job in the store is
// orphaned. Marking it FAILED with an honest reason keeps the system of record
// truthful — never a phantom (PLAN.md §9.1).
func failOrphanedQueued(ctx context.Context, jobs store.Store, bus *apiserver.InMemoryBus, logger *slog.Logger) {
	keys, err := jobs.List(ctx, "job:")
	if err != nil {
		logger.Error("delivery: failed to list jobs for orphan sweep", "err", err)
		return
	}
	for _, key := range keys {
		raw, err := jobs.Get(ctx, key)
		if err != nil {
			continue
		}
		var job corev1.Job
		if err := proto.Unmarshal(raw, &job); err != nil {
			continue
		}
		if job.GetStatus() != corev1.JobStatus_JOB_STATUS_QUEUED {
			continue
		}
		apiserver.RecordJobStatus(ctx, jobs, bus, &corev1.Job{
			Id:          job.GetId(),
			Kind:        job.GetKind(),
			OwnerUserId: job.GetOwnerUserId(),
			Status:      corev1.JobStatus_JOB_STATUS_FAILED,
			Error:       "instance restarted while queued",
		})
	}
}
