package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"
	"testing"

	"github.com/Nem-git/ABCMovies/internal/app"
)

// startApp starts a full app instance (store + gRPC core + web frontend) on
// ephemeral ports and returns its HTTP base URL.
func startApp(t *testing.T) *app.App {
	t.Helper()
	db := filepath.Join(t.TempDir(), "e2e.db")
	a, err := app.Start(t.Context(), app.Config{Bind: "127.0.0.1", HTTPPort: 0, GRPCPort: 0, DBPath: db})
	if err != nil {
		t.Fatalf("app.Start: %v", err)
	}
	t.Cleanup(a.Close)
	return a
}

func httpClient(t *testing.T, base string) *http.Client {
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
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatal(err)
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

func TestBuild1Journey(t *testing.T) {
	t.Run("first-run-admin-and-login", func(t *testing.T) {
		a := startApp(t)
		client := httpClient(t, a.HTTPURL)
		base := a.HTTPURL

		// Fresh instance redirects to /setup.
		resp, err := client.Get(base + "/")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.Request.URL.Path != "/setup" {
			t.Fatalf("GET / on fresh instance = %q, want redirect to /setup", resp.Request.URL.Path)
		}

		// Create the initial admin.
		resp = postJSON(t, client, base+"/api/setup", map[string]string{
			"username": "admin",
			"password": "testpass1234",
		})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST /api/setup status = %d, want 201", resp.StatusCode)
		}

		// After setup, the home page redirects to /login.
		resp, err = client.Get(base + "/")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.Request.URL.Path != "/login" {
			t.Fatalf("GET / after setup = %q, want redirect to /login", resp.Request.URL.Path)
		}

		// Wrong password is rejected.
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
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST /api/login status = %d, want 200", resp.StatusCode)
		}

		// Signed-in user can see the home page.
		resp, err = client.Get(base + "/")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
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