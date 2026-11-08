// Package web implements the browser-facing JSON/HTML frontend. The browser
// never talks gRPC directly; this layer proxies to the core identity service.
package web

import (
	"encoding/json"
	"html"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
)

// SessionCookieName is the HttpOnly cookie holding the session ID.
const SessionCookieName = "abcmovies_session"

// Server wires HTTP routes to the core identity gRPC service.
type Server struct {
	client identityv1.IdentityServiceClient
	mux    *http.ServeMux
}

// NewServer returns a Server using client for core calls.
func NewServer(client identityv1.IdentityServiceClient) *Server {
	s := &Server{client: client, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /setup", s.setupPage)
	s.mux.HandleFunc("POST /api/setup", s.apiSetup)
	s.mux.HandleFunc("GET /login", s.loginPage)
	s.mux.HandleFunc("POST /api/login", s.apiLogin)
	s.mux.HandleFunc("POST /api/logout", s.apiLogout)
	s.mux.HandleFunc("GET /", s.home)
	return s
}

// Handler returns the root http handler.
func (s *Server) Handler() http.Handler { return s.mux }

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type errorResponse struct {
	Error string `json:"error"`
}

var maxCredentialsBody = int64(4 << 10)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func decodeCredentials(w http.ResponseWriter, r *http.Request) (credentials, bool) {
	var c credentials
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "application/json"):
		r.Body = http.MaxBytesReader(w, r.Body, maxCredentialsBody)
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return credentials{}, false
		}
	case strings.HasPrefix(ct, "application/x-www-form-urlencoded"):
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid form")
			return credentials{}, false
		}
		c.Username = r.PostForm.Get("username")
		c.Password = r.PostForm.Get("password")
	default:
		writeError(w, http.StatusUnsupportedMediaType, "unsupported content type")
		return credentials{}, false
	}
	return c, true
}

func isHTTPS(r *http.Request) bool { return r.TLS != nil }

func sessionCookie(id string, https bool) *http.Cookie {
	c := &http.Cookie{
		Name:     SessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if https {
		c.Secure = true
	}
	return c
}

func (s *Server) setupPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, err := s.client.GetStatus(ctx, &identityv1.GetStatusRequest{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "core unavailable")
		return
	}
	if st.GetInitialized() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(setupPageHTML))
}

func (s *Server) apiSetup(w http.ResponseWriter, r *http.Request) {
	c, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	_, err := s.client.SetupAdmin(r.Context(), &identityv1.SetupAdminRequest{Username: c.Username, Password: c.Password})
	if err != nil {
		switch status.Code(err) {
		case codes.InvalidArgument, codes.FailedPrecondition:
			writeError(w, http.StatusBadRequest, status.Convert(err).Message())
		default:
			writeError(w, http.StatusInternalServerError, "could not create admin")
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, err := s.client.GetStatus(ctx, &identityv1.GetStatusRequest{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "core unavailable")
		return
	}
	if !st.GetInitialized() {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(loginPageHTML))
}

func (s *Server) apiLogin(w http.ResponseWriter, r *http.Request) {
	c, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	resp, err := s.client.Login(r.Context(), &identityv1.LoginRequest{Username: c.Username, Password: c.Password})
	if err != nil {
		switch status.Code(err) {
		case codes.Unauthenticated, codes.InvalidArgument:
			writeError(w, http.StatusUnauthorized, "invalid username or password")
		default:
			writeError(w, http.StatusInternalServerError, "login failed")
		}
		return
	}
	http.SetCookie(w, sessionCookie(resp.GetSessionId(), isHTTPS(r)))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "username": resp.GetUsername()})
}

func (s *Server) apiLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil {
		_, _ = s.client.Logout(r.Context(), &identityv1.LogoutRequest{SessionId: c.Value})
	}
	cookie := sessionCookie("", isHTTPS(r))
	cookie.MaxAge = -1
	cookie.Expires = cookie.Expires.AddDate(-1, 0, 0)
	http.SetCookie(w, cookie)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, err := s.client.GetStatus(ctx, &identityv1.GetStatusRequest{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "core unavailable")
		return
	}
	if !st.GetInitialized() {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	c, err := r.Cookie(SessionCookieName)
	if err != nil || c.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	v, err := s.client.ValidateSession(ctx, &identityv1.ValidateSessionRequest{SessionId: c.Value})
	if err != nil || !v.GetValid() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	p := strings.NewReplacer("{{username}}", html.EscapeString(v.GetUsername()))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(p.Replace(homeHTML)))
}