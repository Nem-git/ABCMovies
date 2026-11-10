package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBadSyncCredentialsStaysOnLibrary(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)
	base := "http://" + s.Web.HTTPAddr

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/System/Info":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ServerName":"Bad Sync Mock","Id":"jf-bad-sync-1","Version":"10.1.0"}`))
		case "/Users/AuthenticateByName":
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"Error":"Authentication failed"}`))
		case "/Items":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mock.Close()

	// Setup admin via no-JS.
	resp := postForm(t, client, base+"/setup", url.Values{
		"username": {"admin"},
		"password": {"testpass1234"},
	})
	_ = resp.Body.Close()

	// Connect the mock source (api_key auth works).
	resp = postForm(t, client, base+"/sources/connect", url.Values{
		"connector_id": {"jellyfin"},
		"endpoint":     {mock.URL},
		"display_name": {"Bad Sync Mock"},
		"cred_type":    {"api_key"},
		"api_key":      {"very-secret-key"},
	})
	cbody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	t.Logf("connect status=%d path=%s body=%s", resp.StatusCode, resp.Request.URL.Path, string(cbody))
	if resp.Request.URL.Path != "/sources" {
		t.Fatalf("connect redirected to %q, want /sources", resp.Request.URL.Path)
	}

	// List sources via the API (session cookie is already in the client's jar).
	req, _ := http.NewRequest(http.MethodGet, base+"/api/source", nil)
	listResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	listBody, _ := io.ReadAll(listResp.Body)
	_ = listResp.Body.Close()
	type src struct {
		SourceID string `json:"source_id"`
	}
	var listed struct {
		Sources []src `json:"sources"`
	}
	if err := json.Unmarshal(listBody, &listed); err != nil {
		t.Fatalf("decode source list: %v body=%s", err, listBody)
	}
	if len(listed.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d (%s)", len(listed.Sources), listBody)
	}
	sid := listed.Sources[0].SourceID
	_ = sid

	// Trigger sync with bad password via no-JS form; the source rejects the
	// credentials, and the Library page must re-render with an inline error
	// instead of bouncing the user to /login (and then home).
	resp = postForm(t, client, base+"/library/source/"+sid+"/sync", url.Values{
		"cred_type": {"password"},
		"username":  {"admin"},
		"password":  {"badpassword"},
	})
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("bad sync status=%d, want 200 with inline error. body=%s", resp.StatusCode, body)
	}
	// The page re-renders the library with the sync panel expanded and an inline error.
	if !strings.Contains(string(body), "invalid or expired credentials") {
		t.Fatalf("bad sync missing inline error message. body=%s", body)
	}
	if !strings.Contains(string(body), "<title>Library") {
		t.Fatalf("expected the library page. body=%s", body)
	}
	if !bytes.Contains(body, []byte(`data-source-id="`+sid+`"`)) {
		t.Fatalf("sync panel missing for source. body=%s", body)
	}
	if strings.Contains(resp.Header.Get("Set-Cookie"), "abcmovies_session=;") {
		t.Fatal("bad sync cleared the session cookie")
	}
}
