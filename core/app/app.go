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
	"sync"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/builtin"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"google.golang.org/grpc"
)

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

	// cfg is the loaded instance config, retained so buildSlots and the
	// delivery wiring compose exactly what build loaded.
	cfg *config.Config

	// slots holds the composed provider-slot layer when buildSlots has been
	// called; nil until then.
	slots *SlotRuntime
	// delivery is the composed delivery engine, armed onto the API service
	// once buildSlots has run; nil until then.
	delivery *delivery.Engine

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

	r := registry.NewInProcess()
	if _, err := r.Admit("builtin", builtin.New()); err != nil {
		r.Close()
		_ = closeStores(stores)
		return nil, fmt.Errorf("registry: %w", err)
	}

	bus := apiserver.NewInMemoryBus()
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

	return &Stack{
		service:         srv,
		session:         &sessionSeam{session: session},
		internalSession: session,
		bind:            cfg.Core.API.Bind,
		stores:          stores,
		registry:        r,
		bus:             bus,
		cfg:             cfg,
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
// and the library are unarmed.
func (s *Stack) Slots() *SlotRuntime { return s.slots }

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
	if s.slots == nil {
		return nil
	}
	return s.slots.Relay
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
	if s.slots != nil {
		return s.slots, nil
	}
	cfg := s.cfg
	if cfg == nil {
		var err error
		cfg, err = config.Load("")
		if err != nil {
			return nil, err
		}
	}
	rt, err := ComposeSlots(ctx, cfg.Slots, cfg.Enrichment, cfg.Library,
		s.registry, s.stores.SourceCache, s.stores.MetadataCache, s.stores.Vault, s.stores.Cache, s.bus, logger, cfg.Policy)
	if err != nil {
		return nil, err
	}
	if err := s.armDelivery(rt, logger); err != nil {
		rt.Bus.Close()
		return nil, err
	}
	s.slots = rt
	return rt, nil
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
	if s.slots != nil && s.slots.Scheduler != nil {
		done := s.schedulerDone
		go func() {
			defer close(done)
			s.slots.Scheduler.Run(runCtx)
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
	if s.slots != nil {
		s.slots.Bus.Close()
	}
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
