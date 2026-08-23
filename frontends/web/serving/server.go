package serving

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	"github.com/nem-git/abcmovies/core/app"
	apiv1connect "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1/apiv1connect"
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
// ("" for defaults). The caller owns closing the server.
func New(configPath string, logger *slog.Logger) (*Server, error) {
	stack, err := app.Build(configPath, logger)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	path, handler := apiv1connect.NewCoreServiceHandler(
		&coreServiceAdapter{srv: stack.Service()},
		connect.WithInterceptors(authInterceptor{session: stack.Auth()}),
	)
	mux.Handle(path, handler)
	mux.Handle("POST /debug/job", debugJobHandler{stack: stack})
	mux.Handle("GET /debug/capabilities", debugCapabilitiesHandler{stack: stack})
	mux.Handle("/", staticHandler())

	return &Server{mux: mux, stack: stack}, nil
}

// Handler returns the HTTP handler serving the API and the static page.
func (s *Server) Handler() http.Handler { return s.mux }

// BindAddress returns the configured API bind address.
func (s *Server) BindAddress() string { return s.stack.BindAddress() }

// Close releases every resource the composed stack holds.
func (s *Server) Close() { s.stack.Close() }
