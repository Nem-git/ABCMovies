package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
)

// TestLogsNeverCarryCredentials drives the credential-handling paths with
// sentinel secrets (bad connect, bad sync, bad play, good play through the
// upstream stream proxy, failing sync) and fails if any sentinel shows up in
// slog's output. Bodies not leaking credential material is asserted elsewhere;
// this test pins the logs to the same standard.
//
// Today the app logs little on the expected-failure paths by design; the
// value of the check is as a guardrail, not as proof of existing coverage.
func TestLogsNeverCarryCredentials(t *testing.T) {
	var logs bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(orig) })

	s := startStack(t)
	client := httpClient(t)
	base := "http://" + s.Web.HTTPAddr
	mock := newMockJellyfin(t, "jf-logs-id", "Logs Server", "10.10.7")
	mock.SetItems(movieItem("mv-logs-1", "Logged Movie", "2005"))

	// "correct horse" is the mock's valid password: the secret on every
	// success path. The other two are sentinel secrets for failure paths.
	const wrongPassword = "sentinel-wrong-password"
	const apiKey = "sentinel-api-key-abcdef"

	resp := postJSON(t, client, base+"/api/setup", map[string]string{
		"username": "admin",
		"password": "testpass1234",
	})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup status = %d, want 201", resp.StatusCode)
	}

	// Bad password on connect -> 401.
	resp = postJSON(t, client, base+"/api/source/connect", map[string]any{
		"connector_id": "jellyfin",
		"endpoint":     mock.URL,
		"credentials":  map[string]string{"username": "admin", "password": wrongPassword},
	})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad-credentials connect status = %d, want 401", resp.StatusCode)
	}

	// Correct password -> 201; capture the source id.
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
	var conn struct {
		Source struct {
			SourceID string `json:"source_id"`
		} `json:"source"`
	}
	if err := json.Unmarshal(body, &conn); err != nil {
		t.Fatalf("decode connect response: %v", err)
	}
	if conn.Source.SourceID == "" {
		t.Fatalf("connect response missing source_id: %s", body)
	}
	sourceID := conn.Source.SourceID

	// Bad credentials on sync -> 401.
	resp = postJSON(t, client, base+"/api/library/source/"+sourceID+"/sync", map[string]any{
		"credentials": map[string]string{"username": "admin", "password": wrongPassword},
	})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad-credentials sync status = %d, want 401", resp.StatusCode)
	}

	// Good credentials -> 200; primes the session cache.
	resp = syncSource(t, client, base, sourceID)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sync status = %d, want 200", resp.StatusCode)
	}
	titleID, _, _ := firstTitle(t, client, base)

	// Bad credentials on play -> 401.
	code, playBody := playTitle(t, client, base, titleID, map[string]any{
		"credentials": map[string]string{"username": "admin", "password": wrongPassword},
	})
	if code != http.StatusUnauthorized {
		t.Fatalf("bad-credentials play status = %d, want 401; body=%s", code, playBody)
	}

	// An API key body on play -> 200; exercises the API-key path through the logs.
	code, playBody = playTitle(t, client, base, titleID, map[string]any{
		"credentials": map[string]string{"api_key": apiKey},
	})
	if code != http.StatusOK {
		t.Fatalf("api-key play status = %d, want 200; body=%s", code, playBody)
	}

	// Stream once to hit the upstream-stream proxy and its log lines.
	href := streamURLFor(t, client, base, titleID, map[string]any{})
	resp, err := client.Get(base + href)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d, want 200", resp.StatusCode)
	}

	// The source becomes unreachable -> 503 from sync.
	mock.FailItems(true)
	resp = syncSource(t, client, base, sourceID)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("failed sync status = %d, want 503", resp.StatusCode)
	}
	mock.FailItems(false)

	for _, sentinel := range []string{"correct horse", wrongPassword, apiKey} {
		if bytes.Contains(logs.Bytes(), []byte(sentinel)) {
			// Do not print the captured logs: the point is to fail closed.
			t.Fatalf("slog output contained credential material %q", sentinel)
		}
	}
}
