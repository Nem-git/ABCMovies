package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func postForm(t *testing.T, client *http.Client, target string, form url.Values) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestNoJSSetupLoginLogout(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)
	base := "http://" + s.Web.HTTPAddr

	// Setup via native form.
	resp := postForm(t, client, base+"/setup", url.Values{
		"username": {"admin"},
		"password": {"testpass1234"},
	})
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path != "/" {
		t.Fatalf("POST /setup (no-JS) = %d path=%q, want 200 at /", resp.StatusCode, resp.Request.URL.Path)
	}
	if !bytes.Contains(body, []byte("admin")) {
		t.Fatalf("home after no-JS setup missing username, body=%s", body)
	}
	if sessionCookieValue(t, client, base) == "" {
		t.Fatal("no session cookie after no-JS setup")
	}

	// Logout via native form.
	resp = postForm(t, client, base+"/logout", url.Values{})
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/login" {
		t.Fatalf("POST /logout (no-JS) path = %q, want /login", resp.Request.URL.Path)
	}

	// Failed login via native form re-renders with error and preserves username.
	resp = postForm(t, client, base+"/login", url.Values{
		"username": {"admin"},
		"password": {"wrongpassword"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path != "/login" {
		t.Fatalf("failed POST /login (no-JS) = %d path=%q, want 200 at /login", resp.StatusCode, resp.Request.URL.Path)
	}
	if !bytes.Contains(body, []byte("Invalid username or password.")) {
		t.Fatalf("failed login body missing error, body=%q", body)
	}
	// Password must never be echoed.
	if bytes.Contains(body, []byte("wrongpassword")) {
		t.Fatalf("failed login echoed password, body=%q", body)
	}

	// Successful login via native form.
	resp = postForm(t, client, base+"/login", url.Values{
		"username": {"admin"},
		"password": {"testpass1234"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/" || !bytes.Contains(body, []byte("admin")) {
		t.Fatalf("POST /login (no-JS) path=%q body missing admin", resp.Request.URL.Path)
	}

	// Logout again via form, then home redirects to login.
	resp = postForm(t, client, base+"/logout", url.Values{})
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/login" {
		t.Fatalf("second POST /logout path=%q, want /login", resp.Request.URL.Path)
	}
	resp, err := client.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/login" {
		t.Fatalf("GET / after no-JS logout = %q, want /login", resp.Request.URL.Path)
	}
}

func TestNoJSSourcesConnect(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)
	base := "http://" + s.Web.HTTPAddr

	// Fake Jellyfin server (API-key auth only needs System/Info).
	jf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/System/Info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"ServerName":"E2E Server","Id":"jf-e2e-123","Version":"10.0.0"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer jf.Close()

	// Setup via no-JS form.
	resp := postForm(t, client, base+"/setup", url.Values{
		"username": {"admin"},
		"password": {"testpass1234"},
	})
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/" {
		t.Fatalf("setup path=%q", resp.Request.URL.Path)
	}

	// Empty list renders.
	resp, err := client.Get(base + "/sources")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !bytes.Contains(body, []byte("No sources connected yet.")) {
		t.Fatalf("GET /sources missing empty state, body=%q", body)
	}
	// Forms must work without JS.
	if !bytes.Contains(body, []byte(`action="/sources/connect"`)) || !bytes.Contains(body, []byte(`method="post"`)) {
		t.Fatal("sources page missing native connect form")
	}

	// Connect via native form (API key).
	resp = postForm(t, client, base+"/sources/connect", url.Values{
		"connector_id": {"jellyfin"},
		"endpoint":     {jf.URL},
		"display_name": {"Living Room"},
		"cred_type":    {"api_key"},
		"api_key":      {"secret-key"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/sources" {
		t.Fatalf("POST /sources/connect path=%q, want /sources, body=%s", resp.Request.URL.Path, body)
	}
	if !bytes.Contains(body, []byte("Living Room")) || !bytes.Contains(body, []byte(jf.URL)) {
		t.Fatalf("sources list after connect missing entry, body=%q", body)
	}
	if bytes.Contains(body, []byte("secret-key")) {
		t.Fatal("API key echoed in rendered sources page")
	}
	// Discover source id via JSON API for update/duplicate steps.
	// Use GET via client to list.
	resp2, err := client.Get(base + "/api/source")
	if err != nil {
		t.Fatal(err)
	}
	listBody, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	var wrapped struct {
		Sources []struct {
			SourceID string `json:"source_id"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(listBody, &wrapped); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(wrapped.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d (%s)", len(wrapped.Sources), listBody)
	}
	srcID := wrapped.Sources[0].SourceID
	if srcID == "" {
		t.Fatal("empty source_id")
	}

	// Duplicate: connecting the same server again renders the duplicate panel.
	resp = postForm(t, client, base+"/sources/connect", url.Values{
		"connector_id": {"jellyfin"},
		"endpoint":     {jf.URL},
		"display_name": {"Living Room"},
		"cred_type":    {"api_key"},
		"api_key":      {"any-key"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path != "/sources/connect" {
		t.Fatalf("duplicate POST = %d path=%q, want 200 at /sources/connect", resp.StatusCode, resp.Request.URL.Path)
	}
	if !bytes.Contains(body, []byte("This server is already connected")) {
		t.Fatalf("duplicate body missing panel, body=%q", body)
	}
	wantUpdate := fmt.Sprintf("/sources/%s/update", srcID)
	if !bytes.Contains(body, []byte(wantUpdate)) {
		t.Fatalf("duplicate body missing update action %q", wantUpdate)
	}

	// Update display name only (no credentials needed) via native form.
	resp = postForm(t, client, base+"/sources/"+srcID+"/update", url.Values{
		"display_name": {"Updated Name"},
		"cred_type":    {"password"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/sources" {
		t.Fatalf("POST update path=%q, want /sources, body=%s", resp.Request.URL.Path, body)
	}
	if !bytes.Contains(body, []byte("Updated Name")) {
		t.Fatalf("sources after update missing new name, body=%q", body)
	}

	// Failing connect (unreachable) re-renders with an error and preserves endpoint.
	resp = postForm(t, client, base+"/sources/connect", url.Values{
		"connector_id": {"jellyfin"},
		"endpoint":     {"http://127.0.0.1:1"},
		"display_name": {"Broken"},
		"cred_type":    {"api_key"},
		"api_key":      {"k"},
	})
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path != "/sources/connect" {
		t.Fatalf("failed connect = %d path=%q, want 200 at /sources/connect", resp.StatusCode, resp.Request.URL.Path)
	}
	if !bytes.Contains(body, []byte("could not reach the server")) && !bytes.Contains(body, []byte("Could not connect")) {
		t.Fatalf("failed connect body missing error, body=%q", body)
	}

	// Unauthenticated connect redirects to login.
	anon := httpClient(t)
	// Initialize state is already set by previous setup in this stack; an
	// anonymous client should be redirected to /login, not /setup.
	resp = postForm(t, anon, base+"/sources/connect", url.Values{
		"connector_id": {"jellyfin"},
		"endpoint":     {jf.URL},
		"cred_type":    {"api_key"},
		"api_key":      {"k"},
	})
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/login" {
		t.Fatalf("anon connect path=%q, want /login", resp.Request.URL.Path)
	}
}
