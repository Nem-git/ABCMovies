// Package api implements the ABCMovies HTTP API: the single HTTP session and
// cookie edge for the system. Browser pages (usually via the SSR web
// frontend's same-origin proxy) and external frontends call these JSON
// endpoints; the API talks to the gRPC core. The core never sees HTTP or
// cookies, and HTTP callers never hold core credentials.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
)

// SessionCookieName is the HttpOnly cookie holding the session ID.
const SessionCookieName = "abcmovies_session"

const maxCredentialsBody = int64(4 << 10)

// Server wires HTTP routes to the core identity gRPC service.
type Server struct {
	client identityv1.IdentityServiceClient
	mux    *http.ServeMux
}

// NewServer returns a Server using client for core calls.
func NewServer(client identityv1.IdentityServiceClient) *Server {
	s := &Server{client: client, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/status", s.apiStatus)
	s.mux.HandleFunc("POST /api/setup", s.apiSetup)
	s.mux.HandleFunc("POST /api/login", s.apiLogin)
	s.mux.HandleFunc("POST /api/logout", s.apiLogout)
	s.mux.HandleFunc("GET /api/me", s.apiMe)
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

type statusResponse struct {
	Initialized bool `json:"initialized"`
}

type meResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type loginResponse struct {
	OK       bool   `json:"ok"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

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

func decodeCredentials(w http.ResponseWriter, r *http.Request) (credentials, bool) {
	var c credentials
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "expected application/json")
		return credentials{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCredentialsBody)
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return credentials{}, false
	}
	return c, true
}

// apiStatus reports whether the instance has been initialized.
func (s *Server) apiStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.client.GetStatus(r.Context(), &identityv1.GetStatusRequest{})
	if err != nil {
		slog.Error("core status failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{Initialized: st.GetInitialized()})
}

// apiSetup creates the initial admin on an uninitialized instance and signs
// it in (first-run setup behaves like registration).
func (s *Server) apiSetup(w http.ResponseWriter, r *http.Request) {
	c, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	resp, err := s.client.SetupAdmin(r.Context(), &identityv1.SetupAdminRequest{Username: c.Username, Password: c.Password})
	if err != nil {
		switch status.Code(err) {
		case codes.InvalidArgument:
			writeError(w, http.StatusBadRequest, status.Convert(err).Message())
		case codes.FailedPrecondition:
			writeError(w, http.StatusConflict, "instance already initialized")
		default:
			slog.Error("core setup failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "setup failed")
		}
		return
	}
	username := ""
	if profile, err := s.client.GetUser(r.Context(), &identityv1.GetUserRequest{UserId: resp.GetUserId()}); err != nil {
		// Setup already succeeded and a session was created; keep the client signed
		// in rather than returning a failure they cannot recover from cleanly.
		slog.Warn("core get user failed after setup", "err", err, "user_id", resp.GetUserId())
	} else {
		username = profile.GetUsername()
	}
	http.SetCookie(w, sessionCookie(resp.GetSessionId(), isHTTPS(r)))
	writeJSON(w, http.StatusCreated, loginResponse{OK: true, UserID: resp.GetUserId(), Username: username})
}

// apiLogin verifies credentials and sets the session cookie.
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
			slog.Error("core login failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "login failed")
		}
		return
	}
	username := ""
	if profile, err := s.client.GetUser(r.Context(), &identityv1.GetUserRequest{UserId: resp.GetUserId()}); err != nil {
		// Login already succeeded and a session was created; keep the client signed
		// in rather than returning a failure they cannot recover from cleanly.
		slog.Warn("core get user failed after login", "err", err, "user_id", resp.GetUserId())
	} else {
		username = profile.GetUsername()
	}
	http.SetCookie(w, sessionCookie(resp.GetSessionId(), isHTTPS(r)))
	writeJSON(w, http.StatusOK, loginResponse{OK: true, UserID: resp.GetUserId(), Username: username})
}

// apiLogout ends the current session and clears the cookie.
func (s *Server) apiLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil {
		if _, err := s.client.Logout(r.Context(), &identityv1.LogoutRequest{SessionId: c.Value}); err != nil {
			slog.Warn("core logout failed", "err", err)
		}
	}
	cookie := sessionCookie("", isHTTPS(r))
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// apiMe returns the current session's user, or 401.
func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil || c.Value == "" {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	sess, err := s.client.ValidateSession(r.Context(), &identityv1.ValidateSessionRequest{SessionId: c.Value})
	if err != nil {
		slog.Error("core validate session failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	if !sess.GetValid() {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	profile, err := s.client.GetUser(r.Context(), &identityv1.GetUserRequest{UserId: sess.GetUserId()})
	if err != nil {
		slog.Error("core get user failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	writeJSON(w, http.StatusOK, meResponse{UserID: profile.GetUserId(), Username: profile.GetUsername()})
}
