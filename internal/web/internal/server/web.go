// Package server implements the SSR web frontend: server-rendered HTML pages
// (html/template with embedded template files) plus a same-origin /api/*
// proxy to the ABCMovies HTTP API frontend. It never talks to the gRPC core
// directly and owns no session semantics — the HTTP API is the session edge.
// Form posts (setup, login, logout, sources) let users without JS use the
// app: the web layer translates the form to a JSON call to the HTTP API and
// forwards cookies. JavaScript is a progressive enhancement only.
package server

import (
	"bytes"
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
	// Library page (browse/search) context.
	LibraryTitles     []libraryTitle
	LibrarySyncStates []sourceSyncState
	LibraryQuery      string
	LibraryTotalCount int32
	LibraryLimit      int32
	LibraryOffset     int32
	// Precomputed "Showing N–M of T titles." and prev/next page hrefs
	// (computed server-side so the query stays properly encoded).
	LibraryShowing  string
	LibraryPrevHref string
	LibraryNextHref string
	LibraryError    string
	// Per-source sync panel expansion/error for no-JS re-render.
	SyncSourceID string
	SyncError    string
	SyncCredType string
	SyncUsername string
	// Source detail map (source_id -> sync state) for the per-source panel.
	SyncStateBySourceID map[string]sourceSyncState
	// Title detail page context.
	TitleDetail *libraryTitle
	PlayError   string // inline playback error on the title page
	// Player page context.
	Playback *playbackView
	// Error is the setup/login form error text (server-rendered, no-JS).
	Error        string
	FormUsername string
	// ConnectError and UpdateError are the sources form errors.
	ConnectError string
	UpdateError  string
	// RemoveSourceID is the source whose remove section should expand with
	// RemoveError after a failed no-JS remove (mirrors UpdateSourceID).
	RemoveSourceID string
	RemoveError    string
	// Duplicate is the already-connected source, when the duplicate panel
	// should be visible without JS. Nil hides the panel.
	Duplicate *sourceSummary
	// UpdateSourceID is the source whose per-source update section should
	// be expanded on re-render (a failed no-JS update, or a duplicate
	// connect pointing at that source), so the form and error show without
	// another click. Empty leaves every update section collapsed.
	UpdateSourceID string
	// Preserved non-secret form values so a failed no-JS post re-renders with
	// context. Secrets (password, API key) are never preserved or echoed.
	ConnectEndpoint    string
	ConnectDisplayName string
	ConnectUsername    string
	ConnectCredType    string
	UpdateEndpoint     string
	UpdateDisplayName  string
	UpdateUsername     string
	UpdateCredType     string
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
	for _, page := range []string{"setup", "login", "home", "sources", "library", "title", "player"} {
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
	s.mux.HandleFunc("GET /library", s.libraryPage)
	s.mux.HandleFunc("POST /library/source/{source_id}/sync", s.syncPost)
	s.mux.HandleFunc("GET /library/title/{title_id}", s.titleDetail)
	s.mux.HandleFunc("POST /library/title/{title_id}/play", s.playTitle)
	s.mux.HandleFunc("POST /setup", s.setupPost)
	s.mux.HandleFunc("POST /login", s.loginPost)
	s.mux.HandleFunc("POST /logout", s.logoutPost)
	s.mux.HandleFunc("POST /sources/connect", s.connectPost)
	s.mux.HandleFunc("POST /sources/{source_id}/update", s.updatePost)
	s.mux.HandleFunc("POST /sources/{source_id}/remove", s.removePost)
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

type libraryTitle struct {
	TitleID            string `json:"title_id"`
	SourceID           string `json:"source_id"`
	SourceDisplayName  string `json:"source_display_name"`
	ExternalID         string `json:"external_id"`
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	Year               int    `json:"year"`
	Overview           string `json:"overview"`
	Availability       string `json:"availability"`
	AvailabilityDetail string `json:"availability_detail"`
}

type sourceSyncState struct {
	SourceID          string `json:"source_id"`
	SourceDisplayName string `json:"source_display_name"`
	LastOutcome       string `json:"last_outcome"`
	LastSyncedAt      string `json:"last_synced_at,omitempty"`
	Detail            string `json:"detail,omitempty"`
	TitleCount        int    `json:"title_count"`
}

type listLibraryResponse struct {
	Titles     []libraryTitle    `json:"titles"`
	SyncStates []sourceSyncState `json:"sync_states"`
	TotalCount int32             `json:"total_count"`
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

// apiPost performs a server-side POST against the HTTP API. payload is
// marshaled to JSON; nil sends no body (for logout). It forwards the caller's
// Cookie header and captures Set-Cookie values for the caller to forward to
// the browser. Bodies are never logged (they may contain secrets).
func (s *Server) apiPost(r *http.Request, path string, payload any) (int, []byte, []string, error) {
	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("marshal api request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.apiBase.String()+path, bodyReader)
	if err != nil {
		return 0, nil, nil, err
	}
	if cookie := r.Header.Get("Cookie"); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, resp.Header.Values("Set-Cookie"), err
	}
	return resp.StatusCode, respBody, resp.Header.Values("Set-Cookie"), nil
}

func apiErrorMessage(body []byte, fallback string) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && strings.TrimSpace(e.Error) != "" {
		return e.Error
	}
	return fallback
}

func forwardCookies(w http.ResponseWriter, cookies []string) {
	for _, c := range cookies {
		w.Header().Add("Set-Cookie", c)
	}
}

const maxFormBody = int64(4 << 10)

func parseForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBody)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

type credentialsPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sourceCredentials struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
}

type connectPayload struct {
	ConnectorID string             `json:"connector_id"`
	Endpoint    string             `json:"endpoint"`
	DisplayName string             `json:"display_name,omitempty"`
	Credentials *sourceCredentials `json:"credentials,omitempty"`
}

type updatePayload struct {
	Endpoint    string             `json:"endpoint,omitempty"`
	DisplayName string             `json:"display_name,omitempty"`
	Credentials *sourceCredentials `json:"credentials,omitempty"`
}

type connectResponse struct {
	Outcome string        `json:"outcome"`
	Source  sourceSummary `json:"source"`
	Detail  string        `json:"detail,omitempty"`
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
			slog.Error("api list sources: unauthorized (session still validated here)")
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		slog.Error("api list sources failed", "code", code)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	s.render(w, "sources", pageData{Title: "Sources", Username: username, Sources: list.Sources})
}

func (s *Server) setupPost(w http.ResponseWriter, r *http.Request) {
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
	if !parseForm(w, r) {
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		s.render(w, "setup", pageData{Title: "Set up", FormUsername: username, Error: "username and password are required"})
		return
	}
	code, body, cookies, err := s.apiPost(r, "/api/setup", credentialsPayload{Username: username, Password: password})
	if err != nil {
		slog.Error("api setup unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if code == http.StatusCreated {
		forwardCookies(w, cookies)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	slog.Error("api setup failed", "code", code)
	s.render(w, "setup", pageData{
		Title:        "Set up",
		FormUsername: username,
		Error:        apiErrorMessage(body, "Could not create admin."),
	})
}

func (s *Server) loginPost(w http.ResponseWriter, r *http.Request) {
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
	if !parseForm(w, r) {
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		s.render(w, "login", pageData{Title: "Sign in", FormUsername: username, Error: "Invalid username or password."})
		return
	}
	code, body, cookies, err := s.apiPost(r, "/api/login", credentialsPayload{Username: username, Password: password})
	if err != nil {
		slog.Error("api login unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if code == http.StatusOK {
		forwardCookies(w, cookies)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "login", pageData{
		Title:        "Sign in",
		FormUsername: username,
		Error:        apiErrorMessage(body, "Invalid username or password."),
	})
}

func (s *Server) logoutPost(w http.ResponseWriter, r *http.Request) {
	code, _, cookies, err := s.apiPost(r, "/api/logout", nil)
	if err != nil {
		slog.Error("api logout unavailable", "err", err)
	} else {
		forwardCookies(w, cookies)
		_ = code
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// requireSourcesContext returns the signed-in username, or redirects and
// returns ok=false. It mirrors the GET /sources guards.
func (s *Server) requireSourcesContext(w http.ResponseWriter, r *http.Request) (string, bool) {
	initialized, err := s.initialized(r)
	if err != nil {
		slog.Error("api status unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return "", false
	}
	if !initialized {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return "", false
	}
	username, code, err := s.currentUser(r)
	if err != nil {
		slog.Error("api me unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return "", false
	}
	if code != http.StatusOK {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return "", false
	}
	return username, true
}

func (s *Server) listSources(r *http.Request) (listResponse, bool) {
	var list listResponse
	code, err := s.apiGet(r, "/api/source", &list)
	if err != nil || code != http.StatusOK {
		slog.Error("api list sources unavailable", "code", code, "err", err)
		return listResponse{}, false
	}
	return list, true
}

func (s *Server) connectPost(w http.ResponseWriter, r *http.Request) {
	username, ok := s.requireSourcesContext(w, r)
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	connectorID := strings.TrimSpace(r.FormValue("connector_id"))
	if connectorID == "" {
		connectorID = "jellyfin"
	}
	endpoint := strings.TrimSpace(r.FormValue("endpoint"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	credType := strings.TrimSpace(r.FormValue("cred_type"))
	if credType == "" {
		credType = "password"
	}
	var credentials *sourceCredentials
	if credType == "api_key" {
		apiKey := strings.TrimSpace(r.FormValue("api_key"))
		credentials = &sourceCredentials{APIKey: apiKey}
		_ = apiKey
	} else {
		credentials = &sourceCredentials{
			Username: strings.TrimSpace(r.FormValue("username")),
			Password: r.FormValue("password"),
		}
	}
	payload := connectPayload{
		ConnectorID: connectorID,
		Endpoint:    endpoint,
		DisplayName: displayName,
		Credentials: credentials,
	}
	code, body, cookies, err := s.apiPost(r, "/api/source/connect", payload)
	if err != nil {
		slog.Error("api connect unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	forwardCookies(w, cookies)
	if code == http.StatusCreated {
		http.Redirect(w, r, "/sources", http.StatusSeeOther)
		return
	}
	// Duplicate (200 + outcome duplicate) or an error: re-render the sources
	// page with the list, the preserved connect values, and either the
	// duplicate panel or the error.
	list, ok := s.listSources(r)
	if !ok {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	data := pageData{
		Title:              "Sources",
		Username:           username,
		Sources:            list.Sources,
		ConnectEndpoint:    endpoint,
		ConnectDisplayName: displayName,
		ConnectCredType:    credType,
	}
	if credType != "api_key" {
		data.ConnectUsername = strings.TrimSpace(r.FormValue("username"))
	}
	if code == http.StatusOK {
		var cresp connectResponse
		if json.Unmarshal(body, &cresp) == nil && cresp.Outcome == "duplicate" && cresp.Source.SourceID != "" {
			dupe := cresp.Source
			data.Duplicate = &dupe
			// Expand that source's update section in the list so the no-JS
			// flow can complete without re-typing the endpoint.
			data.UpdateSourceID = dupe.SourceID
			data.UpdateEndpoint = dupe.Endpoint
			data.UpdateDisplayName = dupe.DisplayName
			data.UpdateCredType = "password"
			data.UpdateUsername = ""
			s.render(w, "sources", data)
			return
		}
	}
	data.ConnectError = apiErrorMessage(body, "Could not connect. Check the endpoint and credentials and try again.")
	s.render(w, "sources", data)
}

// removePost disconnects a source via a no-JS form: an explicit "yes, remove"
// checkbox is required, the API call carries no credentials, and on success
// the page redirects to /sources (which re-renders without the source).
// On failure the sources page re-renders with the remove section expanded
// and the honest error.
func (s *Server) removePost(w http.ResponseWriter, r *http.Request) {
	username, ok := s.requireSourcesContext(w, r)
	if !ok {
		return
	}
	sourceID := strings.TrimSpace(r.PathValue("source_id"))
	if sourceID == "" {
		http.Error(w, "source_id is required", http.StatusBadRequest)
		return
	}
	if !parseForm(w, r) {
		return
	}
	renderErr := func(msg string) {
		list, ok := s.listSources(r)
		if !ok {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		s.render(w, "sources", pageData{
			Title:          "Sources",
			Username:       username,
			Sources:        list.Sources,
			RemoveSourceID: sourceID,
			RemoveError:    msg,
		})
	}
	if strings.TrimSpace(r.FormValue("confirm")) != "yes" {
		renderErr("Please confirm by ticking the removal checkbox.")
		return
	}
	code, body, cookies, err := s.apiPost(r, "/api/source/"+url.PathEscape(sourceID)+"/remove", nil)
	if err != nil {
		slog.Error("api remove unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	forwardCookies(w, cookies)
	if code == http.StatusOK {
		http.Redirect(w, r, "/sources", http.StatusSeeOther)
		return
	}
	renderErr(apiErrorMessage(body, "Could not remove the source."))
}

func (s *Server) updatePost(w http.ResponseWriter, r *http.Request) {
	username, ok := s.requireSourcesContext(w, r)
	if !ok {
		return
	}
	sourceID := strings.TrimSpace(r.PathValue("source_id"))
	if sourceID == "" {
		http.Error(w, "source_id is required", http.StatusBadRequest)
		return
	}
	if !parseForm(w, r) {
		return
	}
	endpoint := strings.TrimSpace(r.FormValue("endpoint"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	credType := strings.TrimSpace(r.FormValue("cred_type"))
	if credType == "" {
		credType = "password"
	}
	var credentials *sourceCredentials
	if credType == "api_key" {
		if apiKey := strings.TrimSpace(r.FormValue("api_key")); apiKey != "" {
			credentials = &sourceCredentials{APIKey: apiKey}
		}
	} else if pwd := r.FormValue("password"); strings.TrimSpace(pwd) != "" {
		credentials = &sourceCredentials{
			Username: strings.TrimSpace(r.FormValue("username")),
			Password: pwd,
		}
	}
	payload := updatePayload{
		Endpoint:    endpoint,
		DisplayName: displayName,
		Credentials: credentials,
	}
	code, body, cookies, err := s.apiPost(r, "/api/source/"+url.PathEscape(sourceID)+"/update", payload)
	if err != nil {
		slog.Error("api update unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	forwardCookies(w, cookies)
	if code == http.StatusOK {
		http.Redirect(w, r, "/sources", http.StatusSeeOther)
		return
	}
	list, ok := s.listSources(r)
	if !ok {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	// Re-render with that source's update section expanded so the user
	// can correct and resubmit. No duplicate notice: this is an update
	// of an existing source, not a duplicate connect.
	data := pageData{
		Title:             "Sources",
		Username:          username,
		Sources:           list.Sources,
		UpdateSourceID:    sourceID,
		UpdateEndpoint:    endpoint,
		UpdateDisplayName: displayName,
		UpdateCredType:    credType,
		UpdateError:       apiErrorMessage(body, "Could not update. If the endpoint changed, provide credentials for the new server and try again."),
	}
	if credType != "api_key" {
		data.UpdateUsername = strings.TrimSpace(r.FormValue("username"))
	}
	s.render(w, "sources", data)
}
