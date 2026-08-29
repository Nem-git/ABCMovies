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
	"crypto/cipher"
	"fmt"
	"log/slog"

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

	// configPath is retained so BuildSlots can re-load the caller's slot
	// configuration over the same stack.
	configPath string

	// slots holds the composed provider-slot layer when BuildSlots has been
	// called; nil until then.
	slots *SlotRuntime
	// delivery is the composed delivery engine, armed onto the API service
	// once BuildSlots has run; nil until then.
	delivery *delivery.Engine
}

// Build composes the full core. configPath selects the instance config:
// "" loads defaults; a path applies that YAML over defaults, treating a
// missing file as defaults. The caller owns closing the stack.
func Build(configPath string, logger *slog.Logger) (*Stack, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	stores, err := config.BuildStores(context.Background(), cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("stores: %w", err)
	}

	var vaultAEAD cipher.AEAD
	if cfg.Auth.DEKCache == "encrypted-store" {
		vaultAEAD, err = config.VaultAEAD(cfg, logger)
		if err != nil {
			_ = closeStores(stores)
			return nil, fmt.Errorf("dek-cache: %w", err)
		}
	}
	users, tokens, deks, err := config.BuildAuth(stores.Users, stores.Sessions, cfg.Auth.DEKCache, vaultAEAD)
	if err != nil {
		_ = closeStores(stores)
		return nil, fmt.Errorf("auth: %w", err)
	}
	composite, err := config.BuildAuthenticator(cfg.Auth.Methods, users)
	if err != nil {
		_ = closeStores(stores)
		return nil, fmt.Errorf("auth: %w", err)
	}
	session := config.BuildSession(tokens, deks, config.ParseTokenTTL(cfg.Auth.TokenTTL))

	r := registry.NewInProcess()
	if _, err := r.Admit("builtin", builtin.New()); err != nil {
		r.Close()
		_ = closeStores(stores)
		return nil, fmt.Errorf("registry: %w", err)
	}

	bus := apiserver.NewInMemoryBus()
	srv := apiserver.NewServer(bus, stores, composite, session)

	return &Stack{
		service:         srv,
		session:         &sessionSeam{session: session},
		internalSession: session,
		bind:            cfg.Core.API.Bind,
		stores:          stores,
		registry:        r,
		bus:             bus,
		configPath:      configPath,
	}, nil
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

// Slots returns the composed provider-slot layer, or nil when BuildSlots has
// not been called (a bare core). See BuildSlots.
func (s *Stack) Slots() *SlotRuntime { return s.slots }

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

// BuildSlots composes the provider-slot layer over this stack's stores and
// cache config, making the composed slots available through Slots(). It is a
// no-op once slots are already composed. The stack's config path is re-loaded
// so the caller's slot configuration (catalogue, enrichment cadence) applies.
func (s *Stack) BuildSlots(ctx context.Context, logger *slog.Logger) (*SlotRuntime, error) {
	if s.slots != nil {
		return s.slots, nil
	}
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return nil, err
	}
	rt, err := ComposeSlots(ctx, cfg.Slots, cfg.Enrichment,
		s.registry, s.stores.SourceCache, s.stores.MetadataCache, s.stores.Vault, logger)
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

// Close releases every resource the composed stack holds.
func (s *Stack) Close() {
	if s.delivery != nil {
		s.delivery.Close()
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
