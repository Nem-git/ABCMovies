// Package web implements the SSR web frontend: server-rendered HTML pages
// (html/template with embedded template files) plus a same-origin /api/*
// proxy to the ABCMovies HTTP API frontend. It never talks to the gRPC core
// directly and owns no session semantics — the HTTP API is the session edge.
package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

//go:embed templates/*.html
var templates embed.FS

type pageData struct {
	Title    string
	Username string
	Sources  []sourceSummary
}

// Server wires HTTP pages to the ABCMovies HTTP API frontend.
type Server struct {
	apiBase *url.URL
	pages   map[string]*template.Template
	client  *http.Client
	proxy   *httputil.ReverseProxy
	mux     *http.ServeMux
}

// NewServer returns a Server that proxies/renders against the HTTP API at
// apiAddr, e.g. "http://127.0.0.1:8096".
func NewServer(apiAddr string) (*Server, error) {
	base, err := url.Parse(strings.TrimRight(apiAddr, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid api address %q", apiAddr)
	}

	pages := map[string]*template.Template{}
	for _, page := range []string{"setup", "login", "home", "sources"} {
		tmpl, err := template.ParseFS(templates, "templates/layout.html", "templates/"+page+".html")
		if err != nil {
			return nil, fmt.Errorf("parse template %q: %w", page, err)
		}
		pages[page] = tmpl
	}

	s := &Server{
		apiBase: base,
		pages:   pages,
		client:  &http.Client{Timeout: 5 * time.Second},
		mux:     http.NewServeMux(),
	}
	s.proxy = httputil.NewSingleHostReverseProxy(base)
	s.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("proxy to api failed", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream service unavailable"}`))
	}

	s.mux.Handle("/api/", s.proxy)
	s.mux.HandleFunc("GET /setup", s.setupPage)
	s.mux.HandleFunc("GET /login", s.loginPage)
	s.mux.HandleFunc("GET /{$}", s.home)
	s.mux.HandleFunc("GET /sources", s.sourcesPage)
	return s, nil
}

// Handler returns the root http handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) render(w http.ResponseWriter, page string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages[page].Execute(w, data); err != nil {
		slog.Error("render template", "page", page, "err", err)
		// Headers already sent; nothing safe to do but log.
	}
}

type statusResponse struct {
	Initialized bool `json:"initialized"`
}

type meResponse struct {
	Username string `json:"username"`
}

type sourceSummary struct {
	SourceID           string `json:"source_id"`
	ConnectorID        string `json:"connector_id"`
	VerifiedExternalID string `json:"verified_external_id"`
	Endpoint           string `json:"endpoint"`
	DisplayName        string `json:"display_name"`
	Status             string `json:"status"`
	StatusDetail       string `json:"status_detail,omitempty"`
}

type listResponse struct {
	Sources []sourceSummary `json:"sources"`
}

// apiGet performs a server-side GET against the HTTP API, forwarding the
// caller's Cookie header so the API (the session edge) can resolve the session.
func (s *Server) apiGet(r *http.Request, path string, out any) (int, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.apiBase.String()+path, nil)
	if err != nil {
		return 0, err
	}
	if cookie := r.Header.Get("Cookie"); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if out != nil && resp.StatusCode < 300 && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode api response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// initialized checks the instance's setup state via the HTTP API.
func (s *Server) initialized(r *http.Request) (bool, error) {
	var st statusResponse
	code, err := s.apiGet(r, "/api/status", &st)
	if err != nil || code != http.StatusOK {
		return false, fmt.Errorf("api status: code=%d err=%v", code, err)
	}
	return st.Initialized, nil
}

// currentUser returns the signed-in user (username) via the HTTP API, or false.
func (s *Server) currentUser(r *http.Request) (string, int, error) {
	var me meResponse
	code, err := s.apiGet(r, "/api/me", &me)
	if err != nil {
		return "", 0, err
	}
	return me.Username, code, nil
}

func (s *Server) setupPage(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.initialized(r)
	if err != nil {
		slog.Error("api status unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if initialized {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	s.render(w, "setup", pageData{Title: "Set up"})
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.initialized(r)
	if err != nil {
		slog.Error("api status unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if !initialized {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	// Signed-in users don't need the form — send them home.
	if _, code, err := s.currentUser(r); err != nil {
		slog.Error("api me unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	} else if code == http.StatusOK {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "login", pageData{Title: "Sign in"})
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.initialized(r)
	if err != nil {
		slog.Error("api status unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if !initialized {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	username, code, err := s.currentUser(r)
	if err != nil {
		slog.Error("api me unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if code != http.StatusOK {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	s.render(w, "home", pageData{Title: "ABCMovies", Username: username})
}

func (s *Server) sourcesPage(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.initialized(r)
	if err != nil {
		slog.Error("api status unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if !initialized {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	username, code, err := s.currentUser(r)
	if err != nil {
		slog.Error("api me unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if code != http.StatusOK {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	var list listResponse
	code, err = s.apiGet(r, "/api/source", &list)
	if err != nil {
		slog.Error("api list sources unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if code != http.StatusOK {
		if code == http.StatusUnauthorized {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		slog.Error("api list sources failed", "code", code)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	s.render(w, "sources", pageData{Title: "Sources", Username: username, Sources: list.Sources})
}
