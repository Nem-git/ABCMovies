package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// requireLibraryContext is a thin wrapper for handlers that need the
// signed-in guard (same semantics as sources). It returns the username.
func (s *Server) requireLibraryContext(w http.ResponseWriter, r *http.Request) (string, bool) {
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

func (s *Server) fetchLibrary(r *http.Request, query string, offset int32) (listLibraryResponse, int, error) {
	var out listLibraryResponse
	path := "/api/library"
	if strings.TrimSpace(query) != "" {
		path = "/api/library/search?q=" + url.QueryEscape(query)
		if offset > 0 {
			path += "&offset=" + fmt.Sprintf("%d", offset)
		}
	} else if offset > 0 {
		path = fmt.Sprintf("/api/library?offset=%d", offset)
	}
	code, err := s.apiGet(r, path, &out)
	return out, code, err
}

func (s *Server) libraryPage(w http.ResponseWriter, r *http.Request) {
	username, ok := s.requireLibraryContext(w, r)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	offset := int32(0)
	if v := strings.TrimSpace(r.URL.Query().Get("offset")); v != "" {
		var n int
		if _, err := fmtSscanI32(&n, v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	list, code, err := s.fetchLibrary(r, q, offset)
	if err != nil {
		slog.Error("api library unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if code == http.StatusUnauthorized {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if code != http.StatusOK {
		slog.Error("api library failed", "code", code)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	// Gather sources for the sync controls.
	sources, ok := s.listSources(r)
	if !ok {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	s.render(w, "library", pageData{
		Title:             "Library",
		Username:          username,
		Sources:           sources.Sources,
		LibraryTitles:     list.Titles,
		LibrarySyncStates: list.SyncStates,
		LibraryQuery:      q,
		LibraryTotalCount: list.TotalCount,
		LibraryLimit:      50,
		LibraryOffset:     offset,
	})
}

func (s *Server) titleDetail(w http.ResponseWriter, r *http.Request) {
	username, ok := s.requireLibraryContext(w, r)
	if !ok {
		return
	}
	titleID := strings.TrimSpace(r.PathValue("title_id"))
	if titleID == "" {
		http.Error(w, "title_id is required", http.StatusBadRequest)
		return
	}
	var title libraryTitle
	code, err := s.apiGet(r, "/api/library/title/"+url.PathEscape(titleID), &title)
	if err != nil {
		slog.Error("api get title unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if code == http.StatusUnauthorized {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if code != http.StatusOK {
		http.Error(w, "title not found", http.StatusNotFound)
		return
	}
	s.render(w, "title", pageData{Title: title.Name, Username: username, TitleDetail: &title})
}

func (s *Server) syncPost(w http.ResponseWriter, r *http.Request) {
	username, ok := s.requireLibraryContext(w, r)
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
	credType := strings.TrimSpace(r.FormValue("cred_type"))
	if credType == "" {
		credType = "password"
	}
	var credentials *sourceCredentials
	if credType == "api_key" {
		if apiKey := strings.TrimSpace(r.FormValue("api_key")); apiKey != "" {
			credentials = &sourceCredentials{APIKey: apiKey}
		}
	} else {
		credentials = &sourceCredentials{
			Username: strings.TrimSpace(r.FormValue("username")),
			Password: r.FormValue("password"),
		}
	}
	payload := map[string]any{"credentials": credentials}
	code, body, cookies, err := s.apiPost(r, "/api/library/source/"+url.PathEscape(sourceID)+"/sync", payload)
	if err != nil {
		slog.Error("api sync library unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	forwardCookies(w, cookies)
	if code == http.StatusUnauthorized {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if code == http.StatusOK {
		http.Redirect(w, r, "/library", http.StatusSeeOther)
		return
	}
	// Re-render the library page with the sync panel expanded and the error.
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	sources, ok := s.listSources(r)
	if !ok {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	list, code, err := s.fetchLibrary(r, q, 0)
	if err != nil {
		slog.Error("api library unavailable", "err", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	if code == http.StatusUnauthorized {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if code != http.StatusOK {
		slog.Error("api library failed", "code", code)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	data := pageData{
		Title:             "Library",
		Username:          username,
		Sources:           sources.Sources,
		LibraryTitles:     list.Titles,
		LibrarySyncStates: list.SyncStates,
		LibraryQuery:      q,
		LibraryTotalCount: list.TotalCount,
		SyncSourceID:      sourceID,
		SyncError:         apiErrorMessage(body, "Could not sync. Check the endpoint and credentials and try again."),
		SyncCredType:      credType,
	}
	if credType != "api_key" {
		data.SyncUsername = strings.TrimSpace(r.FormValue("username"))
	}
	s.render(w, "library", data)
}

// fmtSscanI32 parses a strictly non-negative integer; returns error on invalid input.
func fmtSscanI32(out *int, s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("expected a non-negative integer")
	}
	*out = n
	return n, nil
}
