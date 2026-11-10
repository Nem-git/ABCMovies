package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Disconnecting a source tombstones it: the source disappears from the
// list, its titles disappear from browse, direct detail pages resolve with
// no play form, and previously minted stream tickets stop working. Re-adding
// the same server revives the source (titles return after a sync).
func TestDisconnectRemoveAndRevive(t *testing.T) {
	s := startStack(t)
	client := httpClient(t)
	base := "http://" + s.Web.HTTPAddr

	mock := newMockJellyfin(t, "jf-remove-e2e-id", "Remove Server", "10.10.7")
	mock.SetItems(movieItem("mv-remove-1", "Killable E2E Movie", "2005"))

	// Setup + connect via the native (no-JS) form.
	resp := postForm(t, client, base+"/setup", url.Values{
		"username": {"admin"},
		"password": {"testpass1234"},
	})
	_ = resp.Body.Close()
	resp = postForm(t, client, base+"/sources/connect", url.Values{
		"connector_id": {"jellyfin"},
		"endpoint":     {mock.URL},
		"display_name": {"Remove Server"},
		"cred_type":    {"password"},
		"username":     {"admin"},
		"password":     {"correct horse"},
	})
	_ = resp.Body.Close()

	// Sync via the native form so the title exists.
	list, err := client.Get(base + "/api/source")
	if err != nil {
		t.Fatal(err)
	}
	listBody, _ := io.ReadAll(list.Body)
	_ = list.Body.Close()
	var wrapped struct {
		Sources []struct {
			SourceID string `json:"source_id"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(listBody, &wrapped); err != nil {
		t.Fatalf("decode source list: %v", err)
	}
	if len(wrapped.Sources) != 1 {
		t.Fatalf("sources = %+v, want 1", wrapped.Sources)
	}
	sourceID := wrapped.Sources[0].SourceID
	resp = postForm(t, client, base+"/library/source/"+sourceID+"/sync", url.Values{
		"cred_type": {"password"},
		"username":  {"admin"},
		"password":  {"correct horse"},
	})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sync via no-JS form = %d, want 200", resp.StatusCode)
	}

	// Find the title and mint a play ticket first (proves tickets work
	// before removal).
	cookie := sessionCookieValue(t, client, base)
	resp = getWithSession(t, base+"/api/library", cookie)
	listBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var library struct {
		Titles []struct {
			TitleID string `json:"title_id"`
			Name    string `json:"name"`
		} `json:"titles"`
	}
	if err := json.Unmarshal(listBody, &library); err != nil {
		t.Fatalf("decode library: %v", err)
	}
	if len(library.Titles) != 1 || library.Titles[0].Name != "Killable E2E Movie" {
		t.Fatalf("library = %+v, want the one synced movie", library.Titles)
	}
	titleID := library.Titles[0].TitleID

	resp = postJSON(t, client, base+"/api/library/title/"+titleID+"/play", map[string]any{})
	playBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("play before remove = %d, want 200; body=%s", resp.StatusCode, playBody)
	}
	var play struct {
		StreamHref string `json:"stream_href"`
	}
	if err := json.Unmarshal(playBody, &play); err != nil {
		t.Fatalf("decode play: %v", err)
	}
	if play.StreamHref == "" {
		t.Fatal("play response missing stream_href")
	}

	// The pre-minted ticket streams fine right now.
	req, err := http.NewRequest(http.MethodGet, base+play.StreamHref, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", "abcmovies_session="+cookie)
	streamResp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("stream before remove = %d, want 200", streamResp.StatusCode)
	}

	// Remove without confirming: the server-side guard re-renders the
	// sources page with an inline error; the source is still listed.
	resp = postForm(t, client, base+"/sources/"+sourceID+"/remove", url.Values{})
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !bytes.Contains(body, []byte("Please confirm")) {
		t.Fatalf("unconfirmed remove missing the inline error: %s", body)
	}
	resp = getWithSession(t, base+"/api/source", cookie)
	listBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !bytes.Contains(listBody, []byte(sourceID)) {
		t.Fatal("unconfirmed remove deleted the source")
	}

	// Remove with confirm: 303 to /sources, source gone from the list.
	resp = postForm(t, client, base+"/sources/"+sourceID+"/remove", url.Values{
		"confirm": {"yes"},
	})
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/sources" {
		t.Fatalf("remove redirect path = %q, want /sources", resp.Request.URL.Path)
	}
	resp = getWithSession(t, base+"/api/source", cookie)
	listBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if bytes.Contains(listBody, []byte(sourceID)) {
		t.Fatalf("source still listed after remove: %s", listBody)
	}
	if bytes.Contains(listBody, []byte("Killable E2E Movie")) {
		t.Fatal("unreachable; the title is not part of the source list")
	}

	// Browse no longer shows the title (tombstoned titles are hidden)...
	resp, err = client.Get(base + "/library")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if bytes.Contains(page, []byte("Killable E2E Movie")) {
		t.Fatal("library still shows the removed source's title")
	}

	// ...but the detail page still resolves by id with the honest notice
	// and no play form.
	req, _ = http.NewRequest(http.MethodGet, base+"/library/title/"+titleID, nil)
	req.Header.Set("Cookie", "abcmovies_session="+cookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	detail, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detail page after remove = %d, want 200", resp.StatusCode)
	}
	if !bytes.Contains(detail, []byte("not currently presented as playable")) {
		t.Fatalf("detail page missing the honest notice: %s", detail)
	}
	if bytes.Contains(detail, []byte(`action="/library/title/`+titleID+`/play"`)) {
		t.Fatal("detail page offered a play form after remove")
	}

	// The previously minted ticket no longer streams.
	req, err = http.NewRequest(http.MethodGet, base+play.StreamHref, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", "abcmovies_session="+cookie)
	streamResp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusNotFound {
		t.Fatalf("stream after remove = %d, want 404 (ticket revoked)", streamResp.StatusCode)
	}

	// Reconnecting the same server revives the tombstone: 303 redirects to
	// /sources (outcome connected, not duplicate), the source is listed,
	// and after a sync the title is playable again.
	resp = postForm(t, client, base+"/sources/connect", url.Values{
		"connector_id": {"jellyfin"},
		"endpoint":     {mock.URL},
		"display_name": {"Remove Server"},
		"cred_type":    {"password"},
		"username":     {"admin"},
		"password":     {"correct horse"},
	})
	_ = resp.Body.Close()
	if resp.Request.URL.Path != "/sources" {
		t.Fatalf("reconnect path = %q, want /sources", resp.Request.URL.Path)
	}
	resp = getWithSession(t, base+"/api/source", cookie)
	listBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !bytes.Contains(listBody, []byte(sourceID)) {
		t.Fatalf("reconnect did not revive the tombstoned source: %s", listBody)
	}

	resp = postForm(t, client, base+"/library/source/"+sourceID+"/sync", url.Values{
		"cred_type": {"password"},
		"username":  {"admin"},
		"password":  {"correct horse"},
	})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("re-sync = %d, want 200", resp.StatusCode)
	}
	resp = getWithSession(t, base+"/api/library", cookie)
	listBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !bytes.Contains(listBody, []byte("Killable E2E Movie")) {
		t.Fatalf("library after revive+sync missing the title: %s", listBody)
	}

	// A second remove of an unknown source re-renders with "not found".
	resp = postForm(t, client, base+"/sources/src_does_not_exist/remove", url.Values{
		"confirm": {"yes"},
	})
	bogus, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(bogus), "not found") {
		t.Fatalf("remove of unknown source missing the honest error: %s", bogus)
	}
}
