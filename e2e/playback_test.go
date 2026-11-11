package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func firstTitle(t *testing.T, client *http.Client, base string) (titleID, kind, availability string) {
	t.Helper()
	resp := getWithSession(t, base+"/api/library", sessionCookieValue(t, client, base))
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", resp.StatusCode, body)
	}
	var list struct {
		Titles []struct {
			TitleID      string `json:"title_id"`
			Kind         string `json:"kind"`
			Availability string `json:"availability"`
		} `json:"titles"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Titles) == 0 {
		t.Fatal("no titles after sync")
	}
	return list.Titles[0].TitleID, list.Titles[0].Kind, list.Titles[0].Availability
}

func playTitle(t *testing.T, client *http.Client, base, titleID string, payload map[string]any) (int, []byte) {
	t.Helper()
	resp := postJSON(t, client, base+"/api/library/title/"+titleID+"/play", payload)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func streamURLFor(t *testing.T, client *http.Client, base, titleID string, payload map[string]any) string {
	t.Helper()
	code, body := playTitle(t, client, base, titleID, payload)
	if code != http.StatusOK {
		t.Fatalf("play status = %d, body=%s", code, body)
	}
	var play struct {
		StreamHref string `json:"stream_href"`
	}
	if err := json.Unmarshal(body, &play); err != nil {
		t.Fatalf("decode play: %v", err)
	}
	return play.StreamHref
}

func TestPlayCredentialHandling(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)
	base := "http://" + s.Web.HTTPAddr
	mock := newMockJellyfin(t, "jf-cold-id", "Cold Cache Server", "10.10.7")
	mock.SetItems(movieItem("mv-cold-1", "Cold Movie", "2000"))
	base, sourceID := setupAndConnect(t, s, client, mock)

	// Sync supplies credentials once and primes the session cache.
	resp := syncSource(t, client, base, sourceID)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sync status = %d", resp.StatusCode)
	}
	titleID, _, _ := firstTitle(t, client, base)

	// Explicit wrong credentials at play time: the source rejects them
	// (the cache does not override request-supplied credentials).
	code, body := playTitle(t, client, base, titleID, map[string]any{
		"credentials": map[string]string{"username": "admin", "password": "wrong password"},
	})
	if code != http.StatusUnauthorized {
		t.Fatalf("play with wrong creds status = %d, want 401; body=%s", code, body)
	}

	// A blank play request reuses the sync-primed session cache.
	code, _ = playTitle(t, client, base, titleID, map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("play with sync-primed cache status = %d, want 200", code)
	}

	// The cache survives: a second blank play also works.
	code, _ = playTitle(t, client, base, titleID, map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("play with cache status = %d, want 200", code)
	}
}

func TestPlayRejectsSeriesAndUnavailable(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)
	mock := newMockJellyfin(t, "jf-series-id", "Series Server", "10.10.7")
	mock.SetItems(seriesItem("sr-1", "A Series", "2010"), movieItem("mv-1", "Gone Movie", "1999"))
	base, sourceID := setupAndConnect(t, s, client, mock)

	resp := syncSource(t, client, base, sourceID)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sync status = %d", resp.StatusCode)
	}

	// Series: clear, honest rejection.
	searchResp := getWithSession(t, base+"/api/library/search?q=A+Series", sessionCookieValue(t, client, base))
	searchBody, _ := io.ReadAll(searchResp.Body)
	_ = searchResp.Body.Close()
	var search struct {
		Titles []struct {
			TitleID string `json:"title_id"`
			Kind    string `json:"kind"`
		} `json:"titles"`
	}
	if err := json.Unmarshal(searchBody, &search); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if len(search.Titles) != 1 || search.Titles[0].Kind != "series" {
		t.Fatalf("search for series returned %+v", search.Titles)
	}
	seriesID := search.Titles[0].TitleID
	code, body := playTitle(t, client, base, search.Titles[0].TitleID, map[string]any{})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("play series status = %d, want 422; body=%s", code, body)
	}
	if !strings.Contains(string(body), "series") {
		t.Fatalf("play series body = %s, want a series explanation", body)
	}

	// The series title page never grows a play form; it explains the limit
	// instead (the title is still available here, before the re-sync below).
	resp = getWithSession(t, base+"/library/title/"+seriesID, sessionCookieValue(t, client, base))
	seriesPage, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("series title page status = %d, want 200; body=%s", resp.StatusCode, seriesPage)
	}
	if !strings.Contains(string(seriesPage), "Series playback is not supported in Build 1 yet") {
		t.Fatalf("series title page missing the series notice: %s", seriesPage)
	}
	if bytes.Contains(seriesPage, []byte(`action="/library/title/`+seriesID+`/play"`)) {
		t.Fatal("series title page offered a play form")
	}

	// Capture the gone title's id before the re-sync (after it, the title is
	// hidden from browse/search per ADR 0006).
	searchResp = getWithSession(t, base+"/api/library/search?q=Gone+Movie", sessionCookieValue(t, client, base))
	searchBody, _ = io.ReadAll(searchResp.Body)
	_ = searchResp.Body.Close()
	if err := json.Unmarshal(searchBody, &search); err != nil {
		t.Fatalf("decode search before re-sync: %v", err)
	}
	if len(search.Titles) != 1 {
		t.Fatalf("expected to find the gone title before re-sync, got %+v", search.Titles)
	}
	unavailableID := search.Titles[0].TitleID

	// Unavailable title: removed from the source, re-sync reusing the
	// session cache (empty credentials), then play must fail honestly.
	mock.SetItems() // nothing left upstream
	resp = postJSON(t, client, base+"/api/library/source/"+sourceID+"/sync", map[string]any{
		"credentials": map[string]string{},
	})
	syncBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("re-sync status = %d (empty creds reuse session cache), body=%s", resp.StatusCode, syncBody)
	}
	// After a successful sync that omits it, the gone title is hidden from
	// browse and search (ADR 0006).
	searchResp = getWithSession(t, base+"/api/library/search?q=Gone+Movie", sessionCookieValue(t, client, base))
	searchBody, _ = io.ReadAll(searchResp.Body)
	_ = searchResp.Body.Close()
	if err := json.Unmarshal(searchBody, &search); err != nil {
		t.Fatalf("decode search after re-sync: %v", err)
	}
	if len(search.Titles) != 0 {
		t.Fatalf("expected the gone title to be hidden from search, got %+v", search.Titles)
	}
	resp = getWithSession(t, base+"/library", sessionCookieValue(t, client, base))
	libraryPage, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if bytes.Contains(libraryPage, []byte("Gone Movie")) {
		t.Fatal("library still lists the gone title after a successful sync")
	}

	// ...but direct id access still resolves with the honest notice, and
	// play stays rejected honestly.
	resp = getWithSession(t, base+"/library/title/"+unavailableID, sessionCookieValue(t, client, base))
	gonePage, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unavailable title page status = %d, want 200; body=%s", resp.StatusCode, gonePage)
	}
	if !strings.Contains(string(gonePage), "not currently presented as playable") {
		t.Fatalf("unavailable title page missing the honest notice: %s", gonePage)
	}
	if bytes.Contains(gonePage, []byte(`action="/library/title/`+unavailableID+`/play"`)) {
		t.Fatal("unavailable title page offered a play form")
	}
	code, body = playTitle(t, client, base, unavailableID, map[string]any{})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("play unavailable status = %d, want 422; body=%s", code, body)
	}

}

func TestStreamTicketBoundaries(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)
	mock := newMockJellyfin(t, "jf-ticket-id", "Ticket Server", "10.10.7")
	mock.SetItems(movieItem("mv-ticket-1", "Ticket Movie", "2005"))
	base, sourceID := setupAndConnect(t, s, client, mock)

	resp := syncSource(t, client, base, sourceID)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sync status = %d", resp.StatusCode)
	}
	titleID, _, _ := firstTitle(t, client, base)
	href := streamURLFor(t, client, base, titleID, map[string]any{})

	// Unknown ticket: uniform 404.
	req, err := http.NewRequest(http.MethodGet, base+"/api/library/title/"+titleID+"/stream?ticket=deadbeefdeadbeef", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(streamResp.Body)
	_ = streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown ticket status = %d, want 404; body=%s", streamResp.StatusCode, body)
	}

	// Ticket minted for this title, but the path names another: 404.
	ticket := strings.TrimPrefix(href, "/api/library/title/"+titleID+"/stream?ticket=")
	req, err = http.NewRequest(http.MethodGet, base+"/api/library/title/other-title/stream?ticket="+ticket, nil)
	if err != nil {
		t.Fatal(err)
	}
	streamResp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusNotFound {
		t.Fatalf("mismatched title path status = %d, want 404", streamResp.StatusCode)
	}

	// No session at all: 401.
	anon := httpClient(t)
	req, err = http.NewRequest(http.MethodGet, base+href, nil)
	if err != nil {
		t.Fatal(err)
	}
	streamResp, err = anon.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous stream status = %d, want 401", streamResp.StatusCode)
	}

	// Out-of-range request: upstream 416 passes through.
	req, err = http.NewRequest(http.MethodGet, base+href, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=9999-10000")
	streamResp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("out-of-range stream status = %d, want 416", streamResp.StatusCode)
	}
}
