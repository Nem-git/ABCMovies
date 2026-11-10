package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"net/url"
)

type playRequestPayload struct {
	Ticket     string `json:"ticket"`
	ExpiresAt  string `json:"expires_at"`
	StreamHref string `json:"stream_href"`
}

type playbackView struct {
	TitleID    string
	Name       string
	StreamHref string
}

// playTitle handles POST /library/title/{title_id}/play: it asks the API to
// mint a stream ticket for the signed-in session and renders the player page.
// Credentials are optional in the form — blank fields mean "use this
// session's cached credentials" (entered once per session, memory only).
func (s *Server) playTitle(w http.ResponseWriter, r *http.Request) {
	username, ok := s.requireLibraryContext(w, r)
	if !ok {
		return
	}
	titleID := strings.TrimSpace(r.PathValue("title_id"))
	if titleID == "" {
		http.Error(w, "title_id is required", http.StatusBadRequest)
		return
	}
	if !parseForm(w, r) {
		return
	}
	payload := map[string]any{}
	creds := map[string]any{}
	if strings.TrimSpace(r.FormValue("api_key")) != "" {
		creds["api_key"] = strings.TrimSpace(r.FormValue("api_key"))
	} else if strings.TrimSpace(r.FormValue("username")) != "" || r.FormValue("password") != "" {
		creds["username"] = strings.TrimSpace(r.FormValue("username"))
		creds["password"] = r.FormValue("password")
	}
	if len(creds) > 0 {
		payload["credentials"] = creds
	}

	code, body, cookies, err := s.apiPost(r, "/api/library/title/"+url.PathEscape(titleID)+"/play", payload)
	if err != nil {
		slog.Error("api play title unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	forwardCookies(w, cookies)
	if code != http.StatusOK {
		s.renderTitleDetail(w, r, username, titleID, apiErrorMessage(body, "Playback is not available. Check the source and your credentials, and try again."))
		return
	}
	var resp playRequestPayload
	if err := json.Unmarshal(body, &resp); err != nil || resp.StreamHref == "" {
		slog.Error("api play title returned an unusable response", "err", err)
		s.renderTitleDetail(w, r, username, titleID, "Playback could not be prepared. Try again.")
		return
	}
	// Fetch the title name for the player page; fall back to the id.
	var title libraryTitle
	if code, err := s.apiGet(r, "/api/library/title/"+url.PathEscape(titleID), &title); err != nil || code != http.StatusOK {
		title = libraryTitle{Name: titleID}
	}
	// The player page embeds the ticket-carrying stream URL: never let it be
	// stored by a browser or proxy cache (all other HTML pages are cached the
	// same way today; this one carries a short-lived capability).
	w.Header().Set("Cache-Control", "no-store")
	s.render(w, "player", pageData{
		Title:    "Play · " + title.Name,
		Username: username,
		Playback: &playbackView{TitleID: titleID, Name: title.Name, StreamHref: resp.StreamHref},
	})
}
