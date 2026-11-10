// Playback routes: mint short-lived stream tickets (JSON) and proxy the
// upstream media stream (bytes) for signed-in browsers. Credentials in
// request bodies are input-only (see library.go for the rules); the ticket
// carries no credentials, lives only in core memory, and is bound to the
// actor's session. The stream proxy passes Range headers through so seeking
// works, and returns no-store responses.
package server

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
)

// registerLibraryPlaybackRoutes wires the playback routes onto mux.
func (s *Server) registerLibraryPlaybackRoutes() {
	s.mux.HandleFunc("POST /api/library/title/{title_id}/play", s.apiLibraryPlay)
	s.mux.HandleFunc("GET /api/library/title/{title_id}/stream", s.apiLibraryStream)
}

type playTicketResponse struct {
	Ticket     string `json:"ticket"`
	ExpiresAt  string `json:"expires_at"` // RFC3339 UTC
	StreamHref string `json:"stream_href"`
}

func buildPlayTicketRequest(actor, titleID string, c *libraryCredentials) *libraryv1.CreatePlayTicketRequest {
	req := &libraryv1.CreatePlayTicketRequest{ActorUserId: actor, TitleId: titleID}
	if c == nil {
		return req // no credentials in body: core reuses the session cache
	}
	if apiKey := strings.TrimSpace(c.APIKey); apiKey != "" {
		req.Credentials = &libraryv1.CreatePlayTicketRequest_ApiKey{ApiKey: apiKey}
		return req
	}
	req.Credentials = &libraryv1.CreatePlayTicketRequest_Password{Password: &libraryv1.PasswordCredentials{
		Username: strings.TrimSpace(c.Username), Password: c.Password,
	}}
	return req
}

// apiLibraryPlay mints a playback ticket for a reachable, available title.
func (s *Server) apiLibraryPlay(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	titleID := strings.TrimSpace(r.PathValue("title_id"))
	if titleID == "" {
		writeError(w, http.StatusBadRequest, "title_id is required")
		return
	}
	var req syncRequest
	if !decodeJSONBody(w, r, maxLibraryBody, &req) {
		return
	}
	// An empty credentials object means "reuse this session's cache".
	var creds *libraryCredentials
	if req.Credentials != nil && (strings.TrimSpace(req.Credentials.APIKey) != "" ||
		strings.TrimSpace(req.Credentials.Password) != "" ||
		strings.TrimSpace(req.Credentials.Username) != "") {
		creds = req.Credentials
	}
	playReq := buildPlayTicketRequest(actor, titleID, creds)
	resp, err := s.libraryClient.CreatePlayTicket(r.Context(), playReq)
	if err != nil {
		s.mapLibraryError(w, err, "core create play ticket failed")
		return
	}
	streamHref := "/api/library/title/" + titleID + "/stream?ticket=" + resp.GetTicket()
	writeJSON(w, http.StatusOK, playTicketResponse{
		Ticket:     resp.GetTicket(),
		ExpiresAt:  resp.GetExpiresAt().AsTime().UTC().Format("2006-01-02T15:04:05Z07:00"),
		StreamHref: streamHref,
	})
}

// apiLibraryStream proxies the upstream media stream for a valid ticket. The
// ticket is session-bound and short-lived; the browser never sees upstream
// credentials. Range requests are passed through so seeking works.
func (s *Server) apiLibraryStream(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	titleID := strings.TrimSpace(r.PathValue("title_id"))
	if titleID == "" {
		writeError(w, http.StatusBadRequest, "title_id is required")
		return
	}
	ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	if ticket == "" {
		writeError(w, http.StatusBadRequest, "ticket is required")
		return
	}
	resp, err := s.libraryClient.ResolvePlayTicket(r.Context(), &libraryv1.ResolvePlayTicketRequest{ActorUserId: actor, Ticket: ticket})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			writeError(w, http.StatusNotFound, "stream link expired or is invalid — reload the page and try again")
		default:
			slog.Error("core resolve play ticket failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "stream service unavailable")
		}
		return
	}
	if resp.GetTitleId() != titleID {
		// Ticket is item-scoped; do not stream a different title with it.
		writeError(w, http.StatusNotFound, "stream link expired or is invalid — reload the page and try again")
		return
	}

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, resp.GetStreamUrl(), nil)
	if err != nil {
		slog.Error("build upstream stream request failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "stream service unavailable")
		return
	}
	upstreamReq.Header.Set("Authorization", resp.GetAuthorization())
	upstreamReq.Header.Set("Accept", "*/*")
	if rng := r.Header.Get("Range"); rng != "" {
		upstreamReq.Header.Set("Range", rng)
	}

	// No timeout: media streams are long-lived. The request context is
	// cancelled when the browser disconnects, which also cancels the
	// upstream fetch.
	upstream, err := streamClient.Do(upstreamReq)
	if err != nil {
		slog.Error("upstream stream failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "could not reach the source — try again")
		return
	}
	defer upstream.Body.Close()

	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if v := upstream.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(upstream.StatusCode)
	_, _ = io.Copy(w, upstream.Body)
}

var streamClient = &http.Client{}
