// Package faketmdb provides a self-contained TMDB API v3 server mock for
// testing the tmdb catalogue slot's LookupTitle and GetMetadata contracts.
// It answers exactly the REST contract the adapter speaks (search, details,
// find) so a behavioural conformance test drives the unmodified adapter end
// to end.
package faketmdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// Server is a self-contained TMDB API v3 mock.
type Server struct {
	mu       sync.Mutex
	srv      *httptest.Server
	handlers map[string]http.HandlerFunc
}

// New starts a fake TMDB server. The handlers map keys are matched against
// the request URL path (without query string); each handler receives the
// full request so it can inspect query params.
func New(handlers map[string]http.HandlerFunc) *Server {
	s := &Server{handlers: handlers}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	return s
}

// URL returns the base URL of the fake server.
func (s *Server) URL() string { return s.srv.URL }

// Close shuts down the fake server.
func (s *Server) Close() { s.srv.Close() }

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	h, ok := s.handlers[r.URL.Path]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	h(w, r)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteJSON marshals a value to a compact JSON string, for embedding in
// fixture response bodies.
func WriteJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// NormalisePath strips query parameters from a URL path for matching.
func NormalisePath(p string) string {
	return strings.Split(p, "?")[0]
}
