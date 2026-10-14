// Package fakejellyfin provides a self-contained Jellyfin server mock for
// testing the Jellyfin adapter's whole-catalogue sync contract. It answers
// exactly the REST contract the adapter speaks (authenticate, index) so a
// behavioral conformance test drives the unmodified adapter end to end.
package fakejellyfin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

// Server is a self-contained Jellyfin serve mock.
type Server struct {
	srv   *httptest.Server
	items []CatalogItem
}

// CatalogItem is the subset of Jellyfin's BaseItemDto the fake index holds.
type CatalogItem struct {
	ID        string            `json:"Id"`
	Type      string            `json:"Type"`
	Name      string            `json:"Name"`
	Year      int               `json:"ProductionYear"`
	Providers map[string]string `json:"ProviderIds"`
}

// New starts a fake Jellyfin server with the given catalogue items.
func New(items []CatalogItem) *Server {
	s := &Server{items: items}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	return s
}

// URL returns the base URL of the fake server.
func (s *Server) URL() string { return s.srv.URL }

// Close shuts down the fake server.
func (s *Server) Close() { s.srv.Close() }

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/Users/AuthenticateByName":
		s.authenticate(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/Items":
		s.index(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) authenticate(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"AccessToken":"fake-token","User":{"Id":"fake-user"}}`))
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "MediaBrowser Token=") {
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	wantKinds := map[string]bool{}
	for _, k := range strings.Split(q.Get("includeItemTypes"), ",") {
		wantKinds[k] = true
	}
	all := make([]CatalogItem, 0, len(s.items))
	for _, it := range s.items {
		if len(wantKinds) == 0 || wantKinds[it.Type] {
			all = append(all, it)
		}
	}
	start, _ := strconv.Atoi(q.Get("startIndex"))
	if limit, err := strconv.Atoi(q.Get("limit")); err == nil && limit > 0 && start+limit < len(all) {
		all = all[start : start+limit]
	} else if start < len(all) {
		all = all[start:]
	} else {
		all = nil
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Items            []CatalogItem `json:"Items"`
		TotalRecordCount int           `json:"TotalRecordCount"`
		StartIndex       int           `json:"StartIndex"`
	}{Items: all, TotalRecordCount: len(s.items), StartIndex: start})
}
