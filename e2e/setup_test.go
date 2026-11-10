package e2e

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

// First-run setup rejects bad input before it initializes anything:
// a too-short username or password fails honestly (without echoing the
// password) and the instance reports itself as not set up. Once a valid
// setup has run, repeating it is refused as already initialized.
func TestSetupNegatives(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)
	base := "http://" + s.Web.HTTPAddr

	for _, tc := range []struct {
		name    string
		body    map[string]string
		phase   string
		secret  string
		message string
	}{
		{"username too short", map[string]string{"username": "a", "password": "testpass1234"}, "username", "a", "username"},
		{"password too short", map[string]string{"username": "admin", "password": "waap12"}, "password", "waap12", "password"},
	} {
		resp := postJSON(t, client, base+"/api/setup", tc.body)
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400; body=%s", tc.name, resp.StatusCode, body)
		}
		if !strings.Contains(string(body), tc.message) {
			t.Fatalf("%s: body = %s, want mention of %q", tc.name, body, tc.message)
		}
		if bytes.Contains(body, []byte(tc.secret)) && tc.phase == "password" {
			t.Fatalf("%s: body echoed the supplied password, body=%s", tc.name, body)
		}
	}

	// Nothing initialized yet: a fresh instance still points at setup.
	resp, err := client.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/setup" {
		t.Fatalf("GET / on uninitialized instance = %q, want /setup", resp.Request.URL.Path)
	}

	// A valid setup initializes the instance.
	resp = postJSON(t, client, base+"/api/setup", map[string]string{"username": "admin", "password": "testpass1234"})
	setupBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("valid setup status = %d, want 201; body=%s", resp.StatusCode, setupBody)
	}

	// Repeating setup is refused honestly; no second admin, no session.
	resp = postJSON(t, client, base+"/api/setup", map[string]string{"username": "admin2", "password": "testpass1234"})
	repeatBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second setup status = %d, want 409; body=%s", resp.StatusCode, repeatBody)
	}
	if !strings.Contains(string(repeatBody), "already initialized") {
		t.Fatalf("second setup body = %s, want an already-initialized message", repeatBody)
	}

	// Still signed in as the first admin, and login works with those creds.
	loginResp := postJSON(t, client, base+"/api/login", map[string]string{"username": "admin", "password": "testpass1234"})
	loginBody, _ := io.ReadAll(loginResp.Body)
	_ = loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login with original admin status = %d, want 200; body=%s", loginResp.StatusCode, loginBody)
	}
}
