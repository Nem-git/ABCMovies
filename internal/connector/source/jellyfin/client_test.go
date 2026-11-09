package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type jellyfinFixture struct {
	server *httptest.Server
	client *Client

	systemID   string
	serverName string
	version    string
	authBody   map[string]string
	authHeader string
}

func newJellyfinFixture(t *testing.T, systemID, serverName, version string) *jellyfinFixture {
	t.Helper()
	f := &jellyfinFixture{
		systemID:   systemID,
		serverName: serverName,
		version:    version,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /Users/AuthenticateByName", func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("unexpected Content-Type: %q", ct)
		}
		f.authHeader = r.Header.Get("Authorization")
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode auth body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.authBody = body
		if body["Username"] == "admin" && body["Pw"] == "correct horse" {
			fmt.Fprint(w, `{"AccessToken":"tok-access-abc"}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"Message":"Authentication failed"}`)
	})
	mux.HandleFunc("GET /System/Info", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		f.authHeader = auth
		if parsed, ok := parseMediaBrowserHeader(auth); !ok {
			t.Errorf("malformed Authorization header: %q", auth)
		} else if parsed["Token"] == "" {
			t.Errorf("Authorization header has no Token: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ServerName":%q,"Id":%q,"Version":%q}`, f.serverName, f.systemID, f.version)
	})
	mux.HandleFunc("GET /System/Info/Public", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected call to /System/Info/Public")
		w.WriteHeader(http.StatusInternalServerError)
	})
	f.server = httptest.NewServer(mux)
	f.client = NewClient()
	t.Cleanup(f.server.Close)
	return f
}

func TestVerifyPassword(t *testing.T) {
	t.Parallel()
	f := newJellyfinFixture(t, "jf-sys-id-1", "My Media Server", "10.10.7")
	info, err := f.client.Verify(context.Background(), f.server.URL, Credentials{Password: &PasswordCredentials{Username: "admin", Password: "correct horse"}})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if info.VerifiedID != "jf-sys-id-1" {
		t.Fatalf("VerifiedID = %q, want jf-sys-id-1", info.VerifiedID)
	}
	if info.DisplayName != "My Media Server" {
		t.Fatalf("DisplayName = %q, want My Media Server", info.DisplayName)
	}
	if info.Endpoint != f.server.URL {
		t.Fatalf("Endpoint = %q, want %q", info.Endpoint, f.server.URL)
	}
	if parsed, ok := parseMediaBrowserHeader(f.authHeader); !ok || parsed["Client"] != "abcmovies" {
		t.Fatalf("auth header missing client %q: %q", "abcmovies", f.authHeader)
	}
	if got := f.authBody["Pw"]; got != "correct horse" {
		t.Fatalf("auth body Pw = %q, want correct horse", got)
	}
	// The password must not be echoed into the browser-visible info.
	if strings.Contains(fmt.Sprintf("%+v", info), "correct horse") {
		t.Fatal("password leaked into ServerInfo")
	}
}

func TestVerifyAPIKey(t *testing.T) {
	t.Parallel()
	f := newJellyfinFixture(t, "jf-sys-id-2", "APIKey Server", "12.1.0")
	info, err := f.client.Verify(context.Background(), f.server.URL, Credentials{APIKey: "very-secret-api-key"})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if info.VerifiedID != "jf-sys-id-2" {
		t.Fatalf("VerifiedID = %q, want jf-sys-id-2", info.VerifiedID)
	}
	if parsed, ok := parseMediaBrowserHeader(f.authHeader); !ok {
		t.Fatalf("malformed Authorization header: %q", f.authHeader)
	} else if parsed["Token"] != "very-secret-api-key" {
		t.Fatalf("api-key path must send the API key as Token, got %q", parsed["Token"])
	}
	if f.authBody != nil {
		t.Fatalf("api-key flow must not call the AuthenticateByName endpoint: %q", f.authBody)
	}
	if strings.Contains(fmt.Sprintf("%+v", info), "very-secret-api-key") {
		t.Fatal("API key leaked into ServerInfo")
	}
}

func TestVerifyBadPassword(t *testing.T) {
	t.Parallel()
	f := newJellyfinFixture(t, "jf-sys-id-3", "Nope", "10.10.7")
	_, err := f.client.Verify(context.Background(), f.server.URL, Credentials{Password: &PasswordCredentials{Username: "admin", Password: "wrong-horse"}})
	if err == nil {
		t.Fatal("Verify succeeded for bad password")
	}
	if err.Kind != ErrKindUnauthorized {
		t.Fatalf("Kind = %q, want %q", err.Kind, ErrKindUnauthorized)
	}
	if err.Retryable {
		t.Fatal("bad password must not be retryable")
	}
}

func TestVerifyInvalidAPIKey(t *testing.T) {
	t.Parallel()
	// A server that rejects everything that is not the known key.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /System/Info", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Authorization"), "known-good-key") {
			fmt.Fprint(w, `{"ServerName":"OK","Id":"jf-sys-id-4","Version":"10.10.0"}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"Message":"Access token is invalid"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	_, err := NewClient().Verify(context.Background(), srv.URL, Credentials{APIKey: "bad-key"})
	if err == nil {
		t.Fatal("Verify succeeded for bad API key")
	}
	if err.Kind != ErrKindUnauthorized {
		t.Fatalf("Kind = %q, want %q", err.Kind, ErrKindUnauthorized)
	}
}

func TestVerifyUnreachable(t *testing.T) {
	t.Parallel()
	// A closed listener on a random port gives a fast, reliable refusal.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	_, err2 := NewClient().Verify(context.Background(), "http://"+addr, Credentials{Password: &PasswordCredentials{Username: "admin", Password: "correct horse"}})
	if err2 == nil {
		t.Fatal("Verify succeeded against a closed port")
	}
	if err2.Kind != ErrKindUnreachable {
		t.Fatalf("Kind = %q, want %q", err2.Kind, ErrKindUnreachable)
	}
	if !err2.Retryable {
		t.Fatal("unreachable must be retryable")
	}
}

func TestVerifyBothCredentialsRejected(t *testing.T) {
	t.Parallel()
	_, err := NewClient().Verify(context.Background(), "https://example.invalid", Credentials{
		Password: &PasswordCredentials{Username: "admin", Password: "correct horse"},
		APIKey:   "key",
	})
	if err == nil {
		t.Fatal("Verify accepted both credential types")
	}
	if err.Kind != ErrKindInvalidEndpoint {
		t.Fatalf("Kind = %q, want %q", err.Kind, ErrKindInvalidEndpoint)
	}
	if strings.Contains(fmt.Sprintf("%+v", err), "correct horse") {
		t.Fatal("password leaked into an error")
	}
}

func TestVerifyErrorsDoNotLeakSecrets(t *testing.T) {
	t.Parallel()
	f := newJellyfinFixture(t, "jf-sys-id-6", "SecretLeakTest", "10.10.7")
	cases := []struct {
		name string
		cred Credentials
	}{
		{"bad password", Credentials{Password: &PasswordCredentials{Username: "admin", Password: "secret-pw"}}},
		{"bad api key", Credentials{APIKey: "secret-api-key"}},
	}
	for _, tc := range cases {
		_, err := f.client.Verify(context.Background(), f.server.URL, tc.cred)
		if err == nil {
			continue
		}
		plain := err.Error()
		for _, leak := range []string{"secret-pw", "secret-api-key", "tok-access-abc"} {
			if strings.Contains(plain, leak) {
				t.Fatalf("%s leaked into error: %s", tc.name, plain)
			}
		}
	}
}

func TestVerifyBasePathPreserved(t *testing.T) {
	t.Parallel()
	// Jellyfin behind a reverse proxy at /jellyfin/ must verify against the
	// sub-path, not the server root.
	_ = newJellyfinFixture(t, "jf-sys-id-7", "Subpath", "10.10.7")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /jellyfin/System/Info", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ServerName":"Subpath","Id":"jf-sys-id-7","Version":"10.10.7"}`)
	})
	mux.HandleFunc("POST /jellyfin/Users/AuthenticateByName", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"AccessToken":"tok-access-abc"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	info, err := NewClient().Verify(context.Background(), srv.URL+"/jellyfin/", Credentials{Password: &PasswordCredentials{Username: "admin", Password: "correct horse"}})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if info.VerifiedID != "jf-sys-id-7" {
		t.Fatalf("VerifiedID = %q, want jf-sys-id-7", info.VerifiedID)
	}
}

// parseMediaBrowserHeader parses `MediaBrowser Key="value", ...` into a map.
// Values are percent-decoded per the MediaBrowser scheme; this keeps the test
// from depending on exact raw formatting while still checking semantics.
func parseMediaBrowserHeader(h string) (map[string]string, bool) {
	if !strings.HasPrefix(h, "MediaBrowser ") {
		return nil, false
	}
	rest := strings.TrimPrefix(h, "MediaBrowser ")
	parts := strings.Split(rest, ",")
	out := map[string]string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return nil, false
		}
		v := strings.TrimSpace(kv[1])
		if len(v) >= 2 && strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
			v = v[1 : len(v)-1]
		} else {
			decoded, err := url.PathUnescape(v)
			if err != nil {
				return nil, false
			}
			v = decoded
			if len(v) >= 2 && strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
				v = v[1 : len(v)-1]
			}
		}
		out[strings.TrimSpace(kv[0])] = v
	}
	return out, true
}
