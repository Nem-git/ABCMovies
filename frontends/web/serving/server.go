package serving

import (
	"context"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	"github.com/nem-git/abcmovies/core/app"
	apiv1connect "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1/apiv1connect"
)

// Debug routes are the web layer's observability surface: read-only JSON
// views over the live instance, plus the authenticated job probe
// (TECHNICAL-DECISIONS.md §1.2). Each path is defined once here — the mux
// registrations and the tests both read these, so a rename is one edit. The
// routes are all under /debug/ so the serving layer can recognise the
// subtree as its own, and none of them exist in the core's gRPC surface.
const (
	routeDebugJob          = "/debug/job"
	routeDebugCapabilities = "/debug/capabilities"
	routeDebugMetadata     = "/debug/metadata"
	routeDebugRegistry     = "/debug/registry"
	routeDebugSourceCache  = "/debug/sourcecache"
	routeDebugEnrichment   = "/debug/enrichment"
)

// Server composes the core in-process and serves it, together with the
// static page, over a single HTTP port. The RPC surface speaks gRPC-Web
// (browsers), plain gRPC (h2c clients), and the Connect protocol; see
// TECHNICAL-DECISIONS.md §1.2.
type Server struct {
	mux   *http.ServeMux
	stack *app.Stack
}

// New composes the full stack with the instance config at configPath
// ("" for defaults), leaving it inert: the embedder decides when the core
// begins running, through Start (PLAN.md §10). The caller owns closing the
// server.
func New(configPath string, logger *slog.Logger) (*Server, error) {
	// Composing the provider-slot layer here is what lets the debug/state
	// views render the live enrichment and registry surfaces. With no slots
	// configured this leaves a healthy, empty composed state.
	stack, err := app.New(context.Background(), configPath, logger)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	path, handler := apiv1connect.NewCoreServiceHandler(
		&coreServiceAdapter{srv: stack.Service()},
		connect.WithInterceptors(authInterceptor{session: stack.Auth()}),
	)
	mux.Handle(path, handler)
	mux.Handle("POST "+routeDebugJob, debugJobHandler{stack: stack})
	mux.Handle("GET "+routeDebugCapabilities, debugCapabilitiesHandler{stack: stack})
	mux.Handle("GET "+routeDebugMetadata, debugMetadataHandler{stack: stack})
	mux.Handle("GET "+routeDebugRegistry, debugRegistryHandler{stack: stack})
	mux.Handle("GET "+routeDebugSourceCache, debugSourceCacheHandler{stack: stack})
	mux.Handle("GET "+routeDebugEnrichment, debugEnrichmentHandler{stack: stack})
	// The media relay serves delivery sessions' staged bytes (PLAN.md §3.6).
	// It is unauthenticated beyond the relay token itself: the token is minted
	// per session per track by the engine and dies with the session, which is
	// the whole authorization story — bearer sessions would just add a second
	// minting step. BuildSlots has run above, so the relay is armed.
	mux.Handle(app.RelayPrefix, stack.RelayHandler())
	mux.Handle("/", staticHandler())

	return &Server{mux: mux, stack: stack}, nil
}

// Handler returns the HTTP handler serving the API and the static page.
func (s *Server) Handler() http.Handler { return s.mux }

// BindAddress returns the configured API bind address.
func (s *Server) BindAddress() string { return s.stack.BindAddress() }

// Start launches the composed core's periodic work (PLAN.md §10). The web
// layer hosts the core but does not control it: New only composes the stack,
// and the embedder decides when — and whether — the core begins running.
func (s *Server) Start(ctx context.Context) error { return s.stack.Start(ctx) }

// Close releases every resource the composed stack holds.
func (s *Server) Close() { s.stack.Close() }
