package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
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
		t.Skip("requires REQ-SRC-01 implementation (#24, DEC-001)")
	})
	t.Run("sync-browse-search-detail", func(t *testing.T) {
		t.Skip("requires REQ-LIB-01/02/03 implementation (#25)")
	})
	t.Run("play-reachable-non-drm-title", func(t *testing.T) {
		t.Skip("requires REQ-PLY-01 implementation (#26, DEC-008)")
	})
}
