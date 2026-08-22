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
	"fmt"
	"log/slog"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/builtin"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/registry"
)

// Session is the authentication surface a serving layer uses: validate a
// presented bearer token and obtain a context carrying the authenticated
// principal's identity (user ID and DEK).
type Session interface {
	// Validate resolves a bearer token to its user ID.
	Validate(token string) (string, error)
	// PrincipalContext returns ctx carrying the user's identity, ready to
	// be handed to the service implementation.
	PrincipalContext(ctx context.Context, userID string) context.Context
}

// Stack is the composed core: the service implementation plus everything
// it needs alive underneath it.
type Stack struct {
	service apiv1.CoreServiceServer
	session Session
	bind    string

	stores   config.Stores
	registry *registry.InProcessRegistry
	bus      *apiserver.InMemoryBus
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

	users, tokens, deks := config.BuildAuth(stores.Users, stores.Sessions)
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
		service:  srv,
		session:  &sessionSeam{session: session},
		bind:     cfg.Core.API.Bind,
		stores:   stores,
		registry: r,
		bus:      bus,
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

// Close releases every resource the composed stack holds.
func (s *Stack) Close() {
	s.bus.Close()
	s.registry.Close()
	_ = closeStores(s.stores)
}

func closeStores(stores config.Stores) error {
	var firstErr error
	for _, c := range []interface{ Close() error }{
		stores.Cache,
		stores.Vault,
		stores.WatchHistory,
		stores.Jobs,
		stores.Sessions,
	} {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// sessionSeam adapts the internal session to the exported seam.
type sessionSeam struct {
	session auth.Session
}

func (s *sessionSeam) Validate(token string) (string, error) {
	return s.session.Validate(token)
}

func (s *sessionSeam) PrincipalContext(ctx context.Context, userID string) context.Context {
	return apiserver.AuthContext(ctx, s.session, userID)
}
