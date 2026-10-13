// Package app is the core's exported bootstrap seam: it composes the full
// stack in-process and hands it to a serving layer (frontends/web) or any
// other embedder. Everything below this package stays internal; the seam
// exposes exactly what a transport needs — the API service implementation
// and bearer-token authentication for the caller's termination.
//
// The gRPC command (core/cmd/abcmovies) composes the same pieces directly;
// this package exists so non-gRPC terminations can reuse one composition
// without widening any internal package (TECHNICAL-DECISIONS.md §1.2).
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/builtin"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/scheduler"
	"github.com/nem-git/abcmovies/core/internal/settings"
	"github.com/nem-git/abcmovies/core/internal/slothealth"
	"github.com/nem-git/abcmovies/core/internal/slotstore"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// operatorPredicate is the bus's routing rule for INSTANCE-audience events:
// only subscribers whose uid carries the operator role receive them. It reads
// the role off the user record, the same rule the API's operator gate uses.
func operatorPredicate(users auth.UserStore) func(uid string) bool {
	return func(uid string) bool {
		username, found := strings.CutPrefix(uid, "user:")
		if !found {
			return false
		}
		data, err := users.GetUser(username)
		return err == nil && data.Role == auth.RoleOperator
	}
}

// wireSlotHealthTransitions connects slot-health transitions to the event bus
// (Batch 5): published to operator subscribers, best-effort — the probe
// record is the system of truth and lands whether or not anyone listens.
func wireSlotHealthTransitions(bus *apiserver.InMemoryBus, slotHP *slothealth.Provider) {
	slotHP.SetTransitionHook(func(tr slothealth.Transition) {
		bus.Publish(&corev1.EventEnvelope{
			Id:       fmt.Sprintf("evt-slothealth-%s-%d", tr.SlotID, tr.At.UnixNano()),
			Type:     corev1.EventType_EVENT_TYPE_SLOT_HEALTH_TRANSITION,
			Audience: corev1.EventAudience_EVENT_AUDIENCE_INSTANCE,
			Payload: &corev1.EventEnvelope_SlotHealthTransition{
				SlotHealthTransition: &corev1.SlotHealthTransitionEvent{
					SlotId:           tr.SlotID,
					Healthy:          tr.Healthy,
					WasHealthy:       tr.WasHealthy,
					LastErrorKind:    tr.LastErrorKind,
					LastErrorMessage: tr.LastErrorMessage,
					TransitionedAt:   timestamppb.New(tr.At),
				},
			},
			EmittedAt: timestamppb.Now(),
		})
	})
}

// Session is the authentication surface a serving layer uses: validate a
// presented bearer token and obtain a context carrying the authenticated
// principal's identity (user ID and session DEK).
type Session interface {
	// Validate resolves a bearer token to its user ID.
	Validate(token string) (string, error)
	// PrincipalContext returns ctx carrying the authenticated identity —
	// the user ID plus this session's DEK, resolved from the same token.
	PrincipalContext(ctx context.Context, userID, token string) context.Context
}

// Stack is the composed core: the service implementation plus everything
// it needs alive underneath it.
type Stack struct {
	service apiv1.CoreServiceServer
	session Session
	// internalSession is the raw auth.Session the gRPC interceptors need. It
	// stays inside app: embedders use Auth() or AuthInterceptors() and never
	// see this type.
	internalSession auth.Session
	bind            string

	stores   config.Stores
	registry *registry.InProcessRegistry
	bus      *apiserver.InMemoryBus

	// slotStore holds the managed slots (D8); nil until armed at boot.
	slotStore *slotstore.Store

	// settings is the instance-settings provider: the single write path for
	// instance settings and the live source the API reads
	// (TECHNICAL-DECISIONS.md §1.45). settingsPatch is the boot-time override
	// resolved from config, applied once when the stack is composed.
	// composite is the authenticator, retained so the pipeline can swap its
	// method set live. All three are nil/zero on a stack built without them.
	settings      *settings.Provider
	settingsPatch settings.Patch
	composite     *auth.CompositeAuthenticator

	// cfg is the loaded instance config, retained so buildSlots and the
	// delivery wiring compose exactly what build loaded.
	cfg *config.Config

	// slots holds the composed provider-slot layer when buildSlots has been
	// called; nil until then. It is swapped under slotsMu by a live admin
	// CRUD reload (D8); every reader goes through currentSlots().
	slots   *SlotRuntime
	slotsMu sync.RWMutex
	// slotSched is the one scheduler the stack owns for its whole life: the
	// boot composition and every reload register and retire jobs on it, so a
	// slot change never leaves a second scheduler nobody runs.
	slotSched *scheduler.Scheduler
	// appliedSlots is the slot set currently composed into slots, kept so a
	// failed reload can restore the composition that was live (D8).
	appliedSlots []*corev1.Slot
	// delivery is the composed delivery engine, armed onto the API service
	// once buildSlots has run; nil until then.
	delivery *delivery.Engine
	// relay is the stack's one media relay for its whole life (PLAN.md §3.6).
	// The delivery engine's sink factory mints tokens into it and the HTTP
	// RelayHandler verifies them against it; a live slot reload re-composes
	// the slot layer over this same relay rather than a fresh one, so a menu
	// staged after a reload still pulls (D8).
	relay *delivery.Relay

	// lifecycle guards the Start/Close pair. Start launches the periodic
	// work — the shared scheduler and the delivery watchdog — under one
	// cancellable context, and Close cancels it and waits (PLAN.md §10). A
	// stack that was never started is a valid, inert composition: the
	// tests' and an embedder's opt-out, and Close is safe either way.
	lifecycleMu   sync.Mutex
	started       bool
	closed        bool
	startCancel   context.CancelFunc
	schedulerDone chan struct{}
}

// build composes the fallible construction half of the stack: configPath
// selects the instance config ("" loads defaults; a missing file means
// defaults), then stores, auth, registry, bus, and the API server come up.
// New is the only exported way in, so construction always composes the slot
// layer over this base too.
func build(configPath string, logger *slog.Logger) (*Stack, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	stores, err := config.BuildStores(context.Background(), cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("stores: %w", err)
	}

	// Instance settings get their own store class: the effective values are
	// seeded from the shipped defaults and overridden by whatever config
	// names, then persisted. The provider is the single write path and the
	// live source the API reads, so a published or enforced value can never
	// drift from the stored one (TECHNICAL-DECISIONS.md §1.45).
	settingsProvider, err := settings.New(stores.Settings, config.DefaultInstanceSettings())
	if err != nil {
		_ = closeStores(stores)
		return nil, fmt.Errorf("settings: %w", err)
	}
	settingsPatch, err := cfg.InstanceSettingsPatch()
	if err != nil {
		_ = closeStores(stores)
		return nil, err
	}

	users, tokens, deks, err := config.BuildAuth(stores.Users, stores.Sessions, cfg.Auth.DEKCache, stores.VaultAEAD)
	if err != nil {
		_ = closeStores(stores)
		return nil, fmt.Errorf("auth: %w", err)
	}
	composite, err := config.BuildAuthenticator(cfg.Auth.Methods, users)
	if err != nil {
		_ = closeStores(stores)
		return nil, fmt.Errorf("auth: %w", err)
	}
	tokenTTL, err := config.ParseTokenTTL(cfg.Auth.TokenTTL)
	if err != nil {
		_ = closeStores(stores)
		return nil, err
	}
	session := config.BuildSession(tokens, deks, tokenTTL)

	// Arm the managed-slot store (D8): slots are loaded from the store at
	// boot and re-loaded on every admin CRUD change.
	slotStore, err := slotstore.New(stores.Slots)
	if err != nil {
		_ = closeStores(stores)
		return nil, fmt.Errorf("slot store: %w", err)
	}

	r := registry.NewInProcess()
	if _, err := r.Admit("builtin", builtin.New()); err != nil {
		r.Close()
		_ = closeStores(stores)
		return nil, fmt.Errorf("registry: %w", err)
	}

	bus := apiserver.NewInMemoryBus()
	// INSTANCE-audience (operational) events, such as slot-health
	// transitions, are routed only to operator subscribers; the predicate
	// reads the role off the user record the same way the API's operator
	// gate does. Without this the bus drops them, fail-closed.
	bus.SetOperatorEntiter(operatorPredicate(users))
	// The instance-wide default for what lowering a cap does to running
	// sessions; an account with no choice of its own inherits it.
	capDefault, err := config.ParseCapChangeDefault(cfg.Delivery.OnCapChange)
	if err != nil {
		r.Close()
		_ = closeStores(stores)
		return nil, err
	}
	// The accounts RPCs validate named members against the directory: a
	// share listed for someone who does not exist is refused, not silently
	// granted to nobody.
	srv := apiserver.NewServer(apiserver.Settings{
		Bus:              bus,
		Stores:           stores,
		Authenticator:    composite,
		Session:          session,
		CapChangeDefault: capDefault,
		UserDirectory:    apiserver.NewUserDirectory(users),
	})
	// The admin slot CRUD reads and writes the managed-slot store (D8).
	srv.SetSlotStore(slotStore)
	srv.SetSlotStore(slotStore)
	// The API reads the effective settings live: GetInstanceInfo,
	// GetInstanceSettings and the owner-id visibility projection always see
	// what the instance enforces, never a boot-frozen copy.
	srv.SetInstanceSettings(settingsProvider)
	if stores.SlotHealth != nil {
		slotHP, err := slothealth.New(stores.SlotHealth)
		if err != nil {
			return nil, fmt.Errorf("slothealth: %w", err)
		}
		wireSlotHealthTransitions(bus, slotHP)
		srv.SetSlotHealth(slotHP)
	}

	return &Stack{
		service:         srv,
		session:         &sessionSeam{session: session},
		internalSession: session,
		bind:            cfg.Core.API.Bind,
		stores:          stores,
		registry:        r,
		bus:             bus,
		cfg:             cfg,
		settings:        settingsProvider,
		settingsPatch:   settingsPatch,
		composite:       composite,
		slotStore:       slotStore,
		slotSched:       scheduler.New(0, logger),
	}, nil
}

// New composes the full core: the fallible construction above plus the
// provider-slot layer, so an embedder either receives a complete, inert
// stack or a clean error — never a half-wired one. The composed stack runs
// no periodic work until Start is called (PLAN.md §10); the caller owns
// closing it.
func New(ctx context.Context, configPath string, logger *slog.Logger) (*Stack, error) {
	s, err := build(configPath, logger)
	if err != nil {
		return nil, err
	}
	if _, err := s.buildSlots(ctx, logger); err != nil {
		s.Close()
		return nil, err
	}
	// Wire the admin-CRUD reload seam (D8): the API layer calls it with the
	// prospective slot set before it persists, so a slot the wiring rejects is
	// never stored and a store write that fails can roll the runtime back.
	if srv, ok := s.service.(*apiserver.Server); ok {
		srv.SetSlotReloader(func(ctx context.Context, slots []*corev1.Slot) error {
			return s.reloadSlots(ctx, slots, logger)
		})
	}
	return s, nil
}

// BindAddress returns the configured API bind address.
func (s *Stack) BindAddress() string { return s.bind }

// Service returns the inbound-API service implementation.
func (s *Stack) Service() apiv1.CoreServiceServer { return s.service }

// EnqueueJob creates a job through the core's job pipeline (persisted and
// announced on the event bus). M0 exposes no public CreateJob RPC, so this
// is how an embedder drives the job/event flow.
func (s *Stack) EnqueueJob(ctx context.Context, job *corev1.Job) error {
	srv, ok := s.service.(*apiserver.Server)
	if !ok {
		return fmt.Errorf("app: unexpected service implementation %T", s.service)
	}
	return srv.CreateJob(ctx, job)
}

// Auth returns the session seam for authenticating requests terminated by
// the embedder's own transport.
func (s *Stack) Auth() Session { return s.session }

// AuthInterceptors returns the gRPC interceptors that authenticate inbound
// requests against the core's session store, for terminations that serve the
// API service over gRPC (core/cmd/abcmovies). The internal session type stays
// inside app — embedders never see it; they either use Auth() for their own
// transport or these ready-built interceptors for gRPC.
func (s *Stack) AuthInterceptors() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	if s.internalSession == nil {
		return nil, nil
	}
	return apiserver.AuthUnaryInterceptor(s.internalSession), apiserver.AuthStreamInterceptor(s.internalSession)
}

// Slots returns the composed provider-slot layer, or nil when the stack was
// assembled without it (New always composes it). When nil, delivery seams
// and the library are unarmed. It reads through the live pointer, so a caller
// after a live admin-CRUD reload sees the recomposed runtime.
func (s *Stack) Slots() *SlotRuntime { return s.currentSlots() }

// currentSlots returns the runtime composed right now. A reload swaps the
// whole runtime under slotsMu rather than mutating the live one, so every
// reader — the read-through delivery and library seams, or an embedder — sees
// one consistent runtime, never a half-retired one.
func (s *Stack) currentSlots() *SlotRuntime {
	s.slotsMu.RLock()
	defer s.slotsMu.RUnlock()
	return s.slots
}

// setSlots publishes a newly composed runtime together with the slot set it
// was built from. The applied set is retained so a failed live reload can
// restore exactly what was live.
func (s *Stack) setSlots(rt *SlotRuntime, applied []*corev1.Slot) {
	s.slotsMu.Lock()
	s.slots = rt
	s.appliedSlots = applied
	s.slotsMu.Unlock()
}

// RelayPrefix is the stable URL prefix under which RelayHandler serves staged
// delivery pulls (PLAN.md §3.6). It is re-exported from the delivery package
// (the single definition) because embedders cannot import core/internal: the
// serving layer mounts the handler here and stamps play-menu URLs with the
// same value.
const RelayPrefix = delivery.RelayPrefix

// Relay returns the composed media relay for serving RelayPrefix pulls
// (PLAN.md §3.6); nil when the stack was assembled without the slot layer
// (New always composes it). Only the built-in device sink grants tickets
// into it, and only its delivery sessions.
func (s *Stack) Relay() *delivery.Relay {
	rt := s.currentSlots()
	if rt == nil {
		return nil
	}
	return rt.Relay
}

// RelayHandler returns an HTTP handler serving staged delivery pulls under
// RelayPrefix (PLAN.md §3.6); nil when the slot layer is uncomposed (New
// always composes it). It gives an embedder an HTTP transport without
// reaching into core internals: the handler is unauthenticated beyond the
// relay token itself — the token is minted per session per track by the
// engine and dies with the session, which is the whole authorization story
// (§3.6).
func (s *Stack) RelayHandler() http.Handler {
	r := s.Relay()
	if r == nil {
		return nil
	}
	return &delivery.RelayHandler{Relay: r}
}

// SlotCapability is one admitted slot's declared contract name and version.
type SlotCapability struct {
	Slot    string
	Name    string
	Version uint32
}

// Capabilities returns what every admitted slot declared at handshake
// (PLAN.md §3.2: nothing is assumed, everything is asked). The result is a
// copy; mutating it does not affect the registry.
func (s *Stack) Capabilities() []SlotCapability {
	snap := s.registry.Snapshot()
	out := make([]SlotCapability, 0, len(snap))
	for slot, info := range snap {
		for _, c := range info.Capabilities {
			out = append(out, SlotCapability{Slot: slot, Name: c.Name, Version: c.Version})
		}
	}
	return out
}

// SealedBlobs is a small sealed-at-rest key/value surface over the vault
// store class. Slots that need to persist a credential (e.g. a provider
// session token) declare their own narrow interface with these exact method
// signatures; Go interfaces are structural, so this type satisfies them
// without the adapter importing anything from the core.
type SealedBlobs struct {
	vault interface {
		Put(ctx context.Context, key string, value []byte) error
		Get(ctx context.Context, key string) ([]byte, error)
	}
}

// Save stores blob under key, sealed by the vault's own cipher.
func (b SealedBlobs) Save(ctx context.Context, key string, blob []byte) error {
	return b.vault.Put(ctx, key, blob)
}

// Load returns the blob under key, or nil when absent — absence is not an
// error for callers restoring credentials.
func (b SealedBlobs) Load(ctx context.Context, key string) ([]byte, error) {
	blob, err := b.vault.Get(ctx, key)
	if err != nil {
		return nil, nil
	}
	return blob, nil
}

// SealedBlobs exposes the vault-backed blob store to embedders and wiring.
func (s *Stack) SealedBlobs() SealedBlobs { return SealedBlobs{vault: s.stores.Vault} }

// NewSealedBlobs builds a SealedBlobs over any store backend — the
// composition root uses it when wiring slots outside a full Stack.
func NewSealedBlobs(vault interface {
	Put(ctx context.Context, key string, value []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
},
) SealedBlobs {
	return SealedBlobs{vault: vault}
}

// buildSlots composes the provider-slot layer over this stack's stores and
// cache config, making it available through Slots(). It is a no-op once the
// slots are already composed. New calls it so construction is
// all-or-nothing.
func (s *Stack) buildSlots(ctx context.Context, logger *slog.Logger) (*SlotRuntime, error) {
	if rt := s.currentSlots(); rt != nil {
		return rt, nil
	}
	slots, err := s.loadSlots(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := s.composeSlotSet(ctx, slots, logger)
	if err != nil {
		return nil, err
	}
	// Delivery is a stack-lifetime concern: the engine, its watchdog, and the
	// read-through seams that point at whatever runtime is live are armed
	// once, here. A later slot reload swaps the runtime the seams read; it
	// never builds a second engine.
	if err := s.armDelivery(rt, logger); err != nil {
		return nil, err
	}
	if err := s.applyInstanceSettings(ctx); err != nil {
		return nil, err
	}
	s.setSlots(rt, slots)
	return rt, nil
}

// loadSlots reads the managed slot set from the store (D8): the store is the
// system of record, so boot and every reload compose exactly what it holds.
// Every kind is listed; the store holds only live slots.
func (s *Stack) loadSlots(ctx context.Context) ([]*corev1.Slot, error) {
	var all []*corev1.Slot
	for _, kind := range slotKinds {
		slots, err := s.slotStore.ListByKind(ctx, kind, "")
		if err != nil {
			return nil, fmt.Errorf("load slots: %w", err)
		}
		all = append(all, slots...)
	}
	return all, nil
}

// slotKinds is every managed slot kind, in the order a list presents them.
var slotKinds = []corev1.SlotKind{
	corev1.SlotKind_SLOT_KIND_PROVIDER,
	corev1.SlotKind_SLOT_KIND_SINK,
	corev1.SlotKind_SLOT_KIND_CATALOGUE,
	corev1.SlotKind_SLOT_KIND_SUBTITLE_SOURCE,
	corev1.SlotKind_SLOT_KIND_DRM,
}

// composeSlotSet builds one SlotRuntime over an explicit slot set, sharing the
// stack's one bus and one scheduler. It does not arm delivery or apply
// instance settings: those are stack-lifetime concerns a live reload must not
// repeat. Jobs registered here land on the running scheduler, so a reload
// adds and retires jobs without a second scheduler nobody services.
func (s *Stack) composeSlotSet(ctx context.Context, slots []*corev1.Slot, logger *slog.Logger) (*SlotRuntime, error) {
	cfg := s.cfg
	if cfg == nil {
		var err error
		cfg, err = config.Load("")
		if err != nil {
			return nil, err
		}
	}
	// One relay for the stack's whole life: every composition (boot and each
	// reload) shares it, so tokens minted by a post-reload sink still verify
	// against the HTTP handler armed at boot.
	if s.relay == nil {
		s.relay = delivery.NewRelay()
	}
	return ComposeSlots(ctx, slots, cfg.Enrichment, cfg.Library,
		s.registry, s.stores.SourceCache, s.stores.MetadataCache, s.stores.Vault, s.stores.Cache,
		s.bus, logger, cfg.Policy, WithScheduler(s.slotSched), WithRelay(s.relay))
}

// runtimeSlotIDs is every registry name a runtime admitted: one per live
// provider slot and one per live catalogue slot. Synthetic user-owned server
// slots appear in Providers (built by SetupProviders), so forgetting these
// retires them too.
func runtimeSlotIDs(rt *SlotRuntime) []string {
	ids := make([]string, 0, len(rt.Providers)+len(rt.Catalogues))
	for _, b := range rt.Providers {
		ids = append(ids, b.Entry.GetId())
	}
	for _, c := range rt.Catalogues {
		ids = append(ids, c.Slot)
	}
	return ids
}

// retireRuntime takes a previously composed runtime out of the live registry
// and scheduler before its replacement is composed: its admitted slots are
// forgotten so the new composition can re-admit the same ids, and its
// recurring jobs leave the shared scheduler. The bus is shared and
// long-lived, so it stays; an in-flight delivery keeps the old runtime's
// resolver and relay through its own session, so retirement never breaks a
// running stream.
func (s *Stack) retireRuntime(rt *SlotRuntime) {
	if rt == nil {
		return
	}
	for _, id := range runtimeSlotIDs(rt) {
		s.registry.Forget(id)
	}
	for _, j := range rt.Jobs {
		s.slotSched.Remove(j.Name)
	}
}

// reloadSlots applies a prospective slot set to the running stack (D8): it
// retires the live runtime's admissions and jobs, composes the new set on the
// shared bus and scheduler, and swaps the runtime readers see. Nothing is
// persisted here — the API layer calls this before it writes the store, so a
// slot the wiring rejects is never stored, and a store write that then fails
// is rolled back by re-applying the set that was live. A compose failure
// restores the previous composition, leaving the instance exactly as it was.
func (s *Stack) reloadSlots(ctx context.Context, slots []*corev1.Slot, logger *slog.Logger) error {
	s.slotsMu.Lock()
	defer s.slotsMu.Unlock()
	old := s.slots
	if old == nil {
		// Composed without a slot layer (a bare stack): nothing live to swap.
		return nil
	}
	s.retireRuntime(old)
	// Snapshot what the registry still admits after retirement so a failed
	// compose can drop whatever it admitted halfway.
	keep := map[string]struct{}{}
	for name := range s.registry.Snapshot() {
		keep[name] = struct{}{}
	}
	newRT, err := s.composeSlotSet(ctx, slots, logger)
	if err != nil {
		s.forgetAdmittedExcept(keep)
		restored, rerr := s.composeSlotSet(ctx, s.appliedSlots, logger)
		if rerr != nil {
			logger.Error("slot reload failed and the previous composition could not be restored",
				"reload_error", err, "restore_error", rerr)
			s.slots = nil
			s.appliedSlots = nil
			return err
		}
		s.slots = restored
		s.syncProbers(restored)
		return err
	}
	s.slots = newRT
	s.appliedSlots = slots
	s.syncProbers(newRT)
	return nil
}

// forgetAdmittedExcept drops every registry entry not in keep. It is the
// cleanup for a compose that admitted some slots and then failed: the partial
// admissions must leave before the previous composition is re-admitted.
func (s *Stack) forgetAdmittedExcept(keep map[string]struct{}) {
	for name := range s.registry.Snapshot() {
		if _, ok := keep[name]; !ok {
			s.registry.Forget(name)
		}
	}
}

// syncProbers replaces the API server's credential-prober set with the one a
// runtime exposes. Probers are the one seam written on reload rather than read
// through: the map is keyed by adapter and replaced wholesale under the
// server's own lock, so a link probe always sees the adapters that are live.
func (s *Stack) syncProbers(rt *SlotRuntime) {
	srv, ok := s.service.(*apiserver.Server)
	if !ok {
		return
	}
	if rt == nil {
		srv.SetProbers(nil)
		return
	}
	srv.SetProbers(rt.Probers)
}

// applyInstanceSettings runs the instance-settings pipeline once, now that
// every consumer exists: it resolves the boot-time config override over the
// stored record and the shipped defaults, validates the result, hands each
// value to its live consumer (the delivery engine's timing, the
// authenticator's method set), and only then persists it
// (TECHNICAL-DECISIONS.md §1.45). The same pipeline is the single write path
// the future UpdateInstanceSettings RPC will call. A rejection refuses boot —
// never a silent fallback to defaults (PLAN.md §2.5). It is a no-op on a
// stack composed without a settings provider (a bare test stack).
func (s *Stack) applyInstanceSettings(ctx context.Context) error {
	if s.settings == nil {
		return nil
	}
	live := settings.LiveApply{
		AuthMethods: s.composite.SetMethods,
		HeartbeatInterval: func(d time.Duration) error {
			s.delivery.SetTiming(0, d, 0)
			return nil
		},
		HeartbeatGrace: func(d time.Duration) error {
			s.delivery.SetTiming(0, 0, d)
			return nil
		},
		SessionTTL: func(d time.Duration) error {
			s.delivery.SetTiming(d, 0, 0)
			return nil
		},
	}
	if _, err := s.settings.Apply(ctx, s.settingsPatch, live); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	return nil
}

// Start launches the stack's periodic work: the shared scheduler and the
// delivery watchdog (PLAN.md §9.1, §10). The embedder calls it explicitly —
// hosting a stack never starts the core's lifecycle as a side effect of a
// request or of construction. Start is one-shot: a second call, or a call
// after Close, is an error rather than a silent second run of the loops.
func (s *Stack) Start(ctx context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed {
		return errors.New("app: stack is closed")
	}
	if s.started {
		return errors.New("app: stack already started")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.startCancel = cancel
	s.started = true

	// The scheduler runs the boot-registered jobs; registering a job after
	// this point starts that job's worker immediately. Its Run returns once
	// every worker has stopped, so the done channel is the shutdown handshake
	// the scheduler never had.
	s.schedulerDone = make(chan struct{})
	// The stack's one scheduler runs every boot job and every job a later
	// reload registers. Start runs it; a reload registers and retires jobs on
	// the running scheduler, so a live change never starts a second one.
	if s.slotSched != nil {
		done := s.schedulerDone
		go func() {
			defer close(done)
			s.slotSched.Run(runCtx)
		}()
	} else {
		close(s.schedulerDone)
	}
	if s.delivery != nil {
		go s.delivery.Watch(runCtx)
	}
	return nil
}

// Close releases every resource the composed stack holds, stopping in
// reverse order whatever Start began and then tearing down construction.
// Safe to call more than once, and safe on a stack that was never started.
func (s *Stack) Close() {
	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		return
	}
	s.closed = true
	cancel := s.startCancel
	schedulerDone := s.schedulerDone
	s.lifecycleMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if s.delivery != nil {
		s.delivery.Close()
	}
	if schedulerDone != nil {
		<-schedulerDone
	}
	// The runtime bus is the stack bus (ComposeSlots is given the stack's
	// bus), so closing it here covers both; Close on the bus is idempotent.
	s.bus.Close()
	s.registry.Close()
	_ = closeStores(s.stores)
}

func closeStores(stores config.Stores) error {
	return config.CloseStores(stores)
}

// sessionSeam adapts the internal session to the exported seam.
type sessionSeam struct {
	session auth.Session
}

func (s *sessionSeam) Validate(token string) (string, error) {
	return s.session.Validate(token)
}

func (s *sessionSeam) PrincipalContext(ctx context.Context, userID, token string) context.Context {
	return apiserver.AuthContext(ctx, s.session, userID, token)
}
