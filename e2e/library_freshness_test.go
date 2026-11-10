package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// setupAndConnect is the shared setup for the freshness failure-state tests:
// first-run admin, connect the mock source, then sync once. Returns the
// client, base URL and source ID. Callers own the mock's items before sync.
func setupAndConnect(t *testing.T, s *stack, client *http.Client, mock *mockJellyfin) (string, string) {
	t.Helper()
	base := "http://" + s.Web.HTTPAddr

	resp := postJSON(t, client, base+"/api/setup", map[string]string{
		"username": "admin",
		"password": "testpass1234",
	})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/setup status = %d, want 201", resp.StatusCode)
	}

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
	return base, sourceID
}

func syncSource(t *testing.T, client *http.Client, base, sourceID string) *http.Response {
	t.Helper()
	resp := postJSON(t, client, base+"/api/library/source/"+sourceID+"/sync", map[string]any{
		"credentials": map[string]string{"username": "admin", "password": "correct horse"},
	})
	return resp
}

func TestLibraryEmptyState(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)

	// The mock ships no items: a successful sync of an empty library is the
	// empty-state acceptance case.
	mock := newMockJellyfin(t, "jf-empty-id", "Empty Server", "10.10.9")
	base, sourceID := setupAndConnect(t, s, client, mock)

	resp := syncSource(t, client, base, sourceID)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sync status = %d, want 200, body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"success"`) {
		t.Fatalf("sync body = %s, want success outcome", body)
	}

	// The library page renders the empty state, not an error or a dead end.
	req, _ := http.NewRequest(http.MethodGet, base+"/library", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /library status = %d, want 200, body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "No titles") {
		t.Fatalf("GET /library missing empty-state message: %s", body)
	}
}

func TestFailedSyncKeepsSnapshotAndDisclosesStale(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)

	mock := newMockJellyfin(t, "jf-stale-id", "Stale Server", "10.10.10")
	mock.SetItems(movieItem("jf-item-1", "Blade Runner", "1982"), seriesItem("jf-item-2", "Severance", "2022"), movieItem("jf-item-3", "The Matrix", "1999"))
	base, sourceID := setupAndConnect(t, s, client, mock)

	resp := syncSource(t, client, base, sourceID)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initial sync status = %d, want 200", resp.StatusCode)
	}

	// The source becomes unreachable; the next sync must fail honestly and
	// keep the prior snapshot.
	mock.FailItems(true)
	resp = syncSource(t, client, base, sourceID)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("failed sync status = %d, want 503, body=%s", resp.StatusCode, body)
	}

	// The API still presents the prior snapshot, recorded as a failed sync.
	req, _ := http.NewRequest(http.MethodGet, base+"/api/library", nil)
	listResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(listResp.Body)
	_ = listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/library status = %d, want 200, body=%s", listResp.StatusCode, body)
	}
	var list libraryListResponse
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode library list: %v", err)
	}
	if list.TotalCount != 3 {
		t.Fatalf("library TotalCount = %d, want 3 (prior snapshot retained)", list.TotalCount)
	}
	if len(list.SyncStates) != 1 {
		t.Fatalf("sync states len = %d, want 1", len(list.SyncStates))
	}
	if list.SyncStates[0].LastOutcome != "failed" {
		t.Fatalf("last sync outcome = %q, want %q", list.SyncStates[0].LastOutcome, "failed")
	}

	// The library page discloses staleness explicitly and still shows titles.
	req, _ = http.NewRequest(http.MethodGet, base+"/library", nil)
	pageResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(pageResp.Body)
	_ = pageResp.Body.Close()
	if pageResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /library status = %d, want 200, body=%s", pageResp.StatusCode, body)
	}
	page := string(body)
	if !strings.Contains(page, "Blade Runner") {
		t.Fatalf("GET /library missing retained title: %s", page)
	}
	if !strings.Contains(page, "out of date") {
		t.Fatalf("GET /library missing stale disclosure: %s", page)
	}
}
