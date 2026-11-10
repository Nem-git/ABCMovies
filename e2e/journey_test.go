package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Nem-git/ABCMovies/internal/app"
)

// stack is a running core + API + web frontend wired to each other.
type stack struct {
	Core *app.Core
	API  *app.API
	Web  *app.Web
}

func startStack(t *testing.T) *stack {
	t.Helper()
	db := filepath.Join(t.TempDir(), "e2e.db")

	core, err := app.StartCore(app.CoreConfig{Bind: "127.0.0.1", Port: 0, DB: db})
	if err != nil {
		t.Fatalf("StartCore: %v", err)
	}
	api, err := app.StartAPI(app.APIConfig{Bind: "127.0.0.1", Port: 0, CoreGRPCAddr: core.GRPCAddr})
	if err != nil {
		core.Close()
		t.Fatalf("StartAPI: %v", err)
	}
	web, err := app.StartWeb(app.WebConfig{Bind: "127.0.0.1", Port: 0, APIBaseURL: "http://" + api.HTTPAddr})
	if err != nil {
		api.Close()
		core.Close()
		t.Fatalf("StartWeb: %v", err)
	}
	t.Cleanup(func() {
		web.Close()
		api.Close()
		core.Close()
	})
	return &stack{Core: core, API: api, Web: web}
}

func httpClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{Jar: jar}
}

func postJSON(t *testing.T, client *http.Client, url string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// sessionCookieValue returns the abcmovies_session cookie in client's jar for base.
func sessionCookieValue(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range client.Jar.Cookies(u) {
		if c.Name == "abcmovies_session" {
			return c.Value
		}
	}
	t.Fatal("no abcmovies_session cookie in jar")
	return ""
}

// getWithSession performs a raw GET with the given session cookie (no jar).
func getWithSession(t *testing.T, url, sessionID string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", "abcmovies_session="+sessionID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

type libraryTitleResponse struct {
	TitleID            string `json:"title_id"`
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	Year               int    `json:"year"`
	Availability       string `json:"availability"`
	AvailabilityDetail string `json:"availability_detail"`
	ExternalID         string `json:"external_id"`
	SourceDisplayName  string `json:"source_display_name"`
}

type librarySyncStateResponse struct {
	SourceDisplayName string `json:"source_display_name"`
	Detail            string `json:"detail,omitempty"`
	LastOutcome       string `json:"last_outcome"`
	TitleCount        int    `json:"title_count"`
}

type libraryListResponse struct {
	Titles     []libraryTitleResponse     `json:"titles"`
	SyncStates []librarySyncStateResponse `json:"sync_states"`
	TotalCount int32                      `json:"total_count"`
}

type librarySearchResponse = libraryListResponse

func TestBuild1Journey(t *testing.T) {
	t.Run("first-run-admin-and-login", func(t *testing.T) {
		s := startStack(t)
		client := httpClient(t)
		base := "http://" + s.Web.HTTPAddr

		// Fresh instance redirects to /setup.
		resp, err := client.Get(base + "/")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.Request.URL.Path != "/setup" {
			t.Fatalf("GET / on fresh instance = %q, want redirect to /setup", resp.Request.URL.Path)
		}

		// Create the initial admin (browser calls same-origin /api/*, proxied to the API).
		// Setup establishes a session like registration (see decisions/tech/0010).
		resp = postJSON(t, client, base+"/api/setup", map[string]string{
			"username": "admin",
			"password": "testpass1234",
		})
		setupBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST /api/setup status = %d, want 201", resp.StatusCode)
		}
		var setupResp map[string]any
		if err := json.Unmarshal(setupBody, &setupResp); err != nil {
			t.Fatalf("decode setup response: %v", err)
		}
		if setupResp["user_id"] == "" || setupResp["username"] != "admin" {
			t.Fatalf("setup response = %v, want user_id and username=admin", setupResp)
		}

		// Setup signs the admin in: home renders signed-in immediately.
		resp, err = client.Get(base + "/")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET / after setup status = %d, want 200 (setup signs in)", resp.StatusCode)
		}
		if !bytes.Contains(body, []byte("admin")) {
			t.Fatalf("GET / body after setup missing username: %q", body)
		}

		// A fresh, unauthenticated client is redirected to /login.
		anonClient := httpClient(t)
		resp, err = anonClient.Get(base + "/")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.Request.URL.Path != "/login" {
			t.Fatalf("GET / with no session = %q, want redirect to /login", resp.Request.URL.Path)
		}

		// Sign out; then wrong password is rejected.
		resp = postJSON(t, client, base+"/api/logout", nil)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /api/logout status = %d, want 200", resp.StatusCode)
		}
		resp = postJSON(t, client, base+"/api/login", map[string]string{
			"username": "admin",
			"password": "wrongpassword",
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("POST /api/login with wrong password status = %d, want 401", resp.StatusCode)
		}

		// Correct credentials log in (cookie jar keeps the session).
		resp = postJSON(t, client, base+"/api/login", map[string]string{
			"username": "admin",
			"password": "testpass1234",
		})
		loginBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /api/login status = %d, want 200", resp.StatusCode)
		}
		var loginResp map[string]any
		if err := json.Unmarshal(loginBody, &loginResp); err != nil {
			t.Fatalf("decode login response: %v", err)
		}
		if loginResp["user_id"] == "" || loginResp["username"] != "admin" {
			t.Fatalf("login response = %v, want user_id and username=admin", loginResp)
		}

		// Signed-in user can see the home page (username rendered server-side).
		resp, err = client.Get(base + "/")
		if err != nil {
			t.Fatal(err)
		}
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET / signed-in status = %d, want 200", resp.StatusCode)
		}
		if !bytes.Contains(body, []byte("admin")) {
			t.Fatalf("GET / body missing username: %q", body)
		}

		// Logout ends the session.
		resp = postJSON(t, client, base+"/api/logout", nil)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /api/logout status = %d, want 200", resp.StatusCode)
		}

		// After logout, home redirects to /login again.
		resp, err = client.Get(base + "/")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.Request.URL.Path != "/login" {
			t.Fatalf("GET / after logout = %q, want redirect to /login", resp.Request.URL.Path)
		}
	})

	t.Run("signed-in-user-and-multiple-sessions", func(t *testing.T) {
		s := startStack(t)
		client := httpClient(t)
		base := "http://" + s.Web.HTTPAddr
		apiBase := "http://" + s.API.HTTPAddr

		// Set up the admin (registration-like: returns session A).
		resp := postJSON(t, client, base+"/api/setup", map[string]string{
			"username": "admin",
			"password": "testpass1234",
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST /api/setup status = %d, want 201", resp.StatusCode)
		}
		sessionA := sessionCookieValue(t, client, base)

		// /login for a signed-in user redirects home instead of rendering the form.
		resp, err := client.Get(base + "/login")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.Request.URL.Path != "/" {
			t.Fatalf("GET /login while signed in = %q, want redirect to /", resp.Request.URL.Path)
		}

		// Re-login with the same account succeeds and issues a new session.
		resp = postJSON(t, client, base+"/api/login", map[string]string{
			"username": "admin",
			"password": "testpass1234",
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /api/login (re-login) status = %d, want 200", resp.StatusCode)
		}
		sessionB := sessionCookieValue(t, client, base)
		if sessionB == sessionA {
			t.Fatal("re-login reused the old session id")
		}

		// A: the previous session is still valid (multiple sessions allowed).
		resp = getWithSession(t, apiBase+"/api/me", sessionA)
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/me with original session status = %d, want 200 (multi-session), body=%s", resp.StatusCode, body)
		}

		// B: the new session is valid too.
		resp = getWithSession(t, apiBase+"/api/me", sessionB)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/me with new session status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("connect-and-verify-jellyfin", func(t *testing.T) {
		s := startStack(t)
		client := httpClient(t)
		base := "http://" + s.Web.HTTPAddr

		// Mock Jellyfin server: password auth + system info with a fixed identity.
		mock := newMockJellyfin(t, "jf-e2e-id-1", "E2E Media", "10.10.7")

		// Set up the admin and stay signed in.
		resp := postJSON(t, client, base+"/api/setup", map[string]string{
			"username": "admin",
			"password": "testpass1234",
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST /api/setup status = %d, want 201", resp.StatusCode)
		}

		// Connect with username/password -> 201 connected.
		resp = postJSON(t, client, base+"/api/source/connect", map[string]any{
			"connector_id": "jellyfin",
			"endpoint":     mock.URL,
			"credentials":  map[string]string{"username": "admin", "password": "correct horse"},
		})
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("connect status = %d, want 201, body=%s", resp.StatusCode, body)
		}
		var connectResp map[string]any
		if err := json.Unmarshal(body, &connectResp); err != nil {
			t.Fatalf("decode connect response: %v", err)
		}
		if connectResp["outcome"] != "connected" {
			t.Fatalf("outcome = %v, want connected", connectResp["outcome"])
		}
		src, _ := connectResp["source"].(map[string]any)
		sourceID, _ := src["source_id"].(string)
		if sourceID == "" {
			t.Fatalf("source_id missing in %v", connectResp)
		}
		if src["verified_external_id"] != "jf-e2e-id-1" {
			t.Fatalf("verified_external_id = %v, want jf-e2e-id-1", src["verified_external_id"])
		}

		// The password must not appear anywhere in the response.
		if bytes.Contains(body, []byte("correct horse")) {
			t.Fatal("password echoed in connect response")
		}

		// List shows the connected source.
		resp = getWithSession(t, base+"/api/source", sessionCookieValue(t, client, base))
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/source status = %d, want 200", resp.StatusCode)
		}
		var listResp struct {
			Sources []struct {
				SourceID string `json:"source_id"`
				Status   string `json:"status"`
			} `json:"sources"`
		}
		if err := json.Unmarshal(body, &listResp); err != nil {
			t.Fatalf("decode list response: %v", err)
		}
		if len(listResp.Sources) != 1 || listResp.Sources[0].SourceID != sourceID {
			t.Fatalf("sources = %+v, want one source %q", listResp.Sources, sourceID)
		}
		if listResp.Sources[0].Status != "active" {
			t.Fatalf("status = %q, want active", listResp.Sources[0].Status)
		}

		// Duplicate: same verified server, different URL spelling (trailing slash)
		// -> 200 duplicate with the same source_id, no second row.
		resp = postJSON(t, client, base+"/api/source/connect", map[string]any{
			"connector_id": "jellyfin",
			"endpoint":     mock.URL + "/",
			"credentials":  map[string]string{"username": "admin", "password": "correct horse"},
		})
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("duplicate connect status = %d, want 200, body=%s", resp.StatusCode, body)
		}
		var dupResp map[string]any
		if err := json.Unmarshal(body, &dupResp); err != nil {
			t.Fatalf("decode duplicate response: %v", err)
		}
		if dupResp["outcome"] != "duplicate" {
			t.Fatalf("outcome = %v, want duplicate", dupResp["outcome"])
		}
		dupSrc, _ := dupResp["source"].(map[string]any)
		if dupSrc["source_id"] != sourceID {
			t.Fatalf("duplicate source_id = %v, want %q", dupSrc["source_id"], sourceID)
		}

		// Bad credentials -> 401, no leak.
		resp = postJSON(t, client, base+"/api/source/connect", map[string]any{
			"connector_id": "jellyfin",
			"endpoint":     mock.URL,
			"credentials":  map[string]string{"username": "admin", "password": "wrong-horse"},
		})
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("bad credentials status = %d, want 401, body=%s", resp.StatusCode, body)
		}
		if bytes.Contains(body, []byte("wrong-horse")) {
			t.Fatal("password echoed in error response")
		}

		// Unreachable endpoint -> 503.
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := l.Addr().String()
		_ = l.Close()
		resp = postJSON(t, client, base+"/api/source/connect", map[string]any{
			"connector_id": "jellyfin",
			"endpoint":     "http://" + addr,
			"credentials":  map[string]string{"username": "admin", "password": "correct horse"},
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("unreachable status = %d, want 503", resp.StatusCode)
		}

		// Update with a different verified server -> 422, no silent reassociation.
		other := newMockJellyfin(t, "jf-e2e-id-2", "Other Server", "10.10.7")
		resp = postJSON(t, client, base+"/api/source/"+sourceID+"/update", map[string]any{
			"endpoint":    other.URL,
			"credentials": map[string]string{"username": "admin", "password": "correct horse"},
		})
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("mismatched update status = %d, want 422, body=%s", resp.StatusCode, body)
		}

		// The source is unchanged after the rejected update.
		resp = getWithSession(t, base+"/api/source/"+sourceID, sessionCookieValue(t, client, base))
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET source after rejected update status = %d, want 200", resp.StatusCode)
		}
		if bytes.Contains(body, []byte(other.URL)) {
			t.Fatalf("source endpoint changed after rejected update: %s", body)
		}

		// API-key connect on a second server -> 201.
		resp = postJSON(t, client, base+"/api/source/connect", map[string]any{
			"connector_id": "jellyfin",
			"endpoint":     other.URL,
			"credentials":  map[string]string{"api_key": "very-secret-api-key"},
		})
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("api-key connect status = %d, want 201, body=%s", resp.StatusCode, body)
		}
		if bytes.Contains(body, []byte("very-secret-api-key")) {
			t.Fatal("API key echoed in connect response")
		}

		// The /sources page renders the connected source for a signed-in user.
		resp, err = client.Get(base + "/sources")
		if err != nil {
			t.Fatal(err)
		}
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /sources status = %d, want 200", resp.StatusCode)
		}
		if !bytes.Contains(body, []byte("E2E Media")) || !bytes.Contains(body, []byte("Other Server")) {
			t.Fatalf("GET /sources body missing connected sources: %s", body)
		}
	})
	t.Run("sync-browse-search-detail", func(t *testing.T) {
		s := startStack(t)
		client := httpClient(t)
		base := "http://" + s.Web.HTTPAddr

		mock := newMockJellyfin(t, "jf-sync-id", "Sync Server", "10.10.7")
		mock.SetItems(movieItem("jf-item-1", "Blade Runner", "1982"), seriesItem("jf-item-2", "Severance", "2022"), movieItem("jf-item-3", "The Matrix", "1999"))

		// Set up the admin and stay signed in.
		resp := postJSON(t, client, base+"/api/setup", map[string]string{
			"username": "admin",
			"password": "testpass1234",
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST /api/setup status = %d, want 201", resp.StatusCode)
		}

		// Connect via password and confirm the outcome.
		resp = postJSON(t, client, base+"/api/source/connect", map[string]any{
			"connector_id": "jellyfin",
			"endpoint":     mock.URL,
			"credentials":  map[string]string{"username": "admin", "password": "correct horse"},
		})
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("connect status = %d, want 201, body=%s", resp.StatusCode, body)
		}
		var connectResp map[string]any
		if err := json.Unmarshal(body, &connectResp); err != nil {
			t.Fatalf("decode connect response: %v", err)
		}
		src, _ := connectResp["source"].(map[string]any)
		sourceID, _ := src["source_id"].(string)
		if sourceID == "" {
			t.Fatal("no source_id in connect response")
		}

		// Trigger sync explicitly (connect does not sync; REQ-LIB-05 ties
		// credentials to each sync until a custody decision lands).
		resp = postJSON(t, client, base+"/api/library/source/"+sourceID+"/sync", map[string]any{
			"credentials": map[string]string{"username": "admin", "password": "correct horse"},
		})
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("sync status = %d, want 200, body=%s", resp.StatusCode, body)
		}

		// The library now lists the items.
		resp, err := client.Get(base + "/api/library")
		if err != nil {
			t.Fatal(err)
		}
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/library status = %d, want 200, body=%s", resp.StatusCode, body)
		}
		var list libraryListResponse
		if err := json.Unmarshal(body, &list); err != nil {
			t.Fatalf("decode library list: %v", err)
		}
		if list.TotalCount != 3 {
			t.Fatalf("library TotalCount = %d, want 3", list.TotalCount)
		}
		if len(list.Titles) != 3 {
			t.Fatalf("library len = %d, want 3", len(list.Titles))
		}

		// Search
		resp, err = client.Get(base + "/api/library/search?q=matrix")
		if err != nil {
			t.Fatal(err)
		}
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/library/search status = %d, body=%s", resp.StatusCode, body)
		}
		var search librarySearchResponse
		if err := json.Unmarshal(body, &search); err != nil {
			t.Fatalf("decode search: %v", err)
		}
		if search.TotalCount != 1 {
			t.Fatalf("search TotalCount = %d, want 1", search.TotalCount)
		}

		// Title detail for the non-DRM movie.
		if len(list.Titles) == 0 || list.Titles[0].TitleID == "" {
			t.Fatal("expected titles in library")
		}
		titleID := list.Titles[0].TitleID
		resp, err = client.Get(base + "/api/library/title/" + titleID)
		if err != nil {
			t.Fatal(err)
		}
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/library/title status = %d, body=%s", resp.StatusCode, body)
		}
		var detail libraryTitleResponse
		if err := json.Unmarshal(body, &detail); err != nil {
			t.Fatalf("decode title detail: %v", err)
		}
		if detail.Name == "" {
			t.Fatal("title detail missing name")
		}
	})
	t.Run("play-reachable-non-drm-title", func(t *testing.T) {
		s := startStack(t)
		client := httpClient(t)
		base := "http://" + s.Web.HTTPAddr

		mock := newMockJellyfin(t, "jf-e2e-play-id", "E2E Play Server", "10.10.7")
		mock.SetItems(movieItem("mv-play-1", "E2E Playable", "2001"))

		resp := postJSON(t, client, base+"/api/setup", map[string]string{
			"username": "admin",
			"password": "testpass1234",
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("setup status = %d", resp.StatusCode)
		}

		resp = postJSON(t, client, base+"/api/source/connect", map[string]any{
			"connector_id": "jellyfin",
			"endpoint":     mock.URL,
			"credentials":  map[string]string{"username": "admin", "password": "correct horse"},
		})
		connBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("connect status = %d", resp.StatusCode)
		}
		var conn struct {
			Source struct {
				SourceID string `json:"source_id"`
			} `json:"source"`
		}
		if err := json.Unmarshal(connBody, &conn); err != nil {
			t.Fatalf("decode connect response: %v", err)
		}
		if conn.Source.SourceID == "" {
			t.Fatalf("connect response missing source_id: %s", connBody)
		}

		// Sync: supplies credentials once for the session and primes the cache.
		resp = postJSON(t, client, base+"/api/library/source/"+conn.Source.SourceID+"/sync", map[string]any{
			"credentials": map[string]string{"username": "admin", "password": "correct horse"},
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("sync status = %d", resp.StatusCode)
		}

		// Find the title id from the list.
		resp = getWithSession(t, base+"/api/library", sessionCookieValue(t, client, base))
		listBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		var list struct {
			Titles []struct {
				TitleID string `json:"title_id"`
				Name    string `json:"name"`
				Kind    string `json:"kind"`
			} `json:"titles"`
		}
		if err := json.Unmarshal(listBody, &list); err != nil {
			t.Fatalf("decode list response: %v", err)
		}
		if len(list.Titles) != 1 {
			t.Fatalf("expected 1 synced title, got %d", len(list.Titles))
		}
		titleID := list.Titles[0].TitleID

		// Play with no credentials in the body: the session cache (primed by
		// sync) kicks in.
		resp = postJSON(t, client, base+"/api/library/title/"+titleID+"/play", map[string]any{})
		playBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("play status = %d, body=%s", resp.StatusCode, playBody)
		}
		var play struct {
			Ticket     string `json:"ticket"`
			StreamHref string `json:"stream_href"`
		}
		if err := json.Unmarshal(playBody, &play); err != nil {
			t.Fatalf("decode play response: %v", err)
		}
		if play.Ticket == "" || play.StreamHref == "" {
			t.Fatalf("play response missing ticket/stream_href: %s", playBody)
		}

		// The video element fetches the stream; the browser carries the session cookie.
		req, err := http.NewRequest(http.MethodGet, base+play.StreamHref, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err = client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		streamBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("stream status = %d, body=%s", resp.StatusCode, streamBody)
		}
		if string(streamBody) != fakeVideoBytes {
			t.Fatalf("stream body = %q, want %q", streamBody, fakeVideoBytes)
		}

		// The title detail page renders the play form (no-JS path).
		resp, err = client.Get(base + "/library/title/" + titleID)
		if err != nil {
			t.Fatal(err)
		}
		titlePage, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("title page status = %d", resp.StatusCode)
		}
		if !bytes.Contains(titlePage, []byte("<video")) && !bytes.Contains(titlePage, []byte(`name="api_key"`)) {
			t.Fatalf("title page missing the play form, body=%s", titlePage)
		}

		// Posting the play form renders the player page with a video element.
		resp = postForm(t, client, base+"/library/title/"+titleID+"/play", url.Values{})
		playerPage, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("play form status = %d, body=%s", resp.StatusCode, playerPage)
		}
		if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("player page Cache-Control = %q, want no-store (page embeds the ticket URL)", cc)
		}
		if !bytes.Contains(playerPage, []byte("<video")) {
			t.Fatalf("player page missing <video>, body=%s", playerPage)
		}
		// The player ships the honest-failure copy and the JS hooks that
		// render it. This asserts the UI is in the page; whether a browser
		// fires the error event is outside this harness's reach.
		if !bytes.Contains(playerPage, []byte("If playback does not start")) {
			t.Fatal("player page missing the honest failure notice")
		}
		if !bytes.Contains(playerPage, []byte("DRM")) {
			t.Fatal("player page missing the protected-content limit")
		}
		if !bytes.Contains(playerPage, []byte("showPlaybackNote")) ||
			!bytes.Contains(playerPage, []byte("addEventListener('error'")) ||
			!bytes.Contains(playerPage, []byte("addEventListener('stalled'")) {
			t.Fatal("player page missing the playback-failure note hooks")
		}
		if bytes.Contains(playerPage, []byte("correct horse")) || bytes.Contains(playerPage, []byte("api_key")) {
			t.Fatal("player page leaked credential material")
		}

		// Range request for seeking: expect 206 and the subrange bytes.
		req, err = http.NewRequest(http.MethodGet, base+play.StreamHref, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Range", "bytes=0-3")
		resp, err = client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		rangeBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusPartialContent {
			t.Fatalf("range stream status = %d, want 206; body=%s", resp.StatusCode, rangeBody)
		}
		if string(rangeBody) != fakeVideoBytes[:4] {
			t.Fatalf("range body = %q, want first 4 bytes of fake video", rangeBody)
		}
	})
}

// mockJellyfin is a minimal Jellyfin HTTP API for e2e tests. It never
// requires a real server and holds no real credentials.
const fakeVideoBytes = "FAKE-MP4-STREAM-PAYLOAD-FOR-E2E-TESTS"

type mockJellyfin struct {
	URL    string
	server *httptest.Server

	items     []string
	failItems bool
}

func newMockJellyfin(t *testing.T, systemID, serverName, version string) *mockJellyfin {
	t.Helper()
	quit := false
	var m *mockJellyfin
	m = &mockJellyfin{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /Users/AuthenticateByName", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string
			Pw       string
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Username != "admin" || body.Pw != "correct horse" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"Message":"Authentication failed"}`)
			return
		}
		fmt.Fprint(w, `{"AccessToken":"tok-e2e-access","User":{"Id":"user-e2e-1"}}`)
	})
	mux.HandleFunc("GET /System/Info", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.Contains(auth, `Token="`) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ServerName":%q,"Id":%q,"Version":%q}`, serverName, systemID, version)
	})
	mux.HandleFunc("GET /Items", func(w http.ResponseWriter, r *http.Request) {
		if m.failItems {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"Message":"upstream unavailable"}`)
			return
		}
		q := r.URL.Query()
		if q.Get("includeItemTypes") != "Movie,Series" || q.Get("fields") != "Overview" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"Items":[%s],"TotalRecordCount":%d}`, strings.Join(m.items, ","), len(m.items))
	})
	mux.HandleFunc("GET /Videos/{itemID}/stream", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.Contains(auth, `Token="`) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		data := []byte(fakeVideoBytes)
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Accept-Ranges", "bytes")
		if rng := r.Header.Get("Range"); strings.HasPrefix(rng, "bytes=") {
			spec := strings.TrimPrefix(rng, "bytes=")
			var start, end int64
			end = int64(len(data)) - 1
			part := strings.SplitN(spec, "-", 2)
			if s, err := strconv.ParseInt(strings.TrimSpace(part[0]), 10, 64); err == nil {
				start = s
			} else {
				start = 0
			}
			if len(part) == 2 {
				if e, err := strconv.ParseInt(strings.TrimSpace(part[1]), 10, 64); err == nil {
					end = e
				}
			}
			if start > end || end >= int64(len(data)) {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", len(data)))
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data[start : end+1])
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data)
	})
	mux.HandleFunc("DELETE /Items", func(w http.ResponseWriter, r *http.Request) {
		// no-op for mock reset support
		q := r.URL.Query()
		_ = q
		m.items = nil
		w.WriteHeader(http.StatusNoContent)
	})
	_ = quit
	m.server = httptest.NewServer(mux)
	m.URL = m.server.URL
	t.Cleanup(m.server.Close)
	return m
}

func movieItem(id, name, year string) string {
	return fmt.Sprintf(`{"Id":%q,"Name":%q,"ProductionYear":%s,"Type":"Movie","Overview":"A test movie."}`, id, name, year)
}

func seriesItem(id, name, year string) string {
	return fmt.Sprintf(`{"Id":%q,"Name":%q,"ProductionYear":%s,"Type":"Series","Overview":"A test series."}`, id, name, year)
}

func (m *mockJellyfin) SetItems(items ...string) {
	m.items = items
}

// FailItems toggles the mock's /Items endpoint to return 503, simulating a
// source that becomes unreachable mid-life. Successful prior syncs remain
// in the snapshot.
func (m *mockJellyfin) FailItems(fail bool) {
	m.failItems = fail
}
