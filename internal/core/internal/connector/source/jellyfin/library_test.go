package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type libraryPage struct {
	Items []string
	Count int // TotalRecordCount to report
}

func newLibraryServer(t *testing.T, includeUserID bool, pages ...libraryPage) (*httptest.Server, *struct{ gotURL []string }) {
	t.Helper()
	seen := &struct{ gotURL []string }{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /Users/AuthenticateByName", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed struct{ Username, Pw string }
		_ = json.Unmarshal(body, &parsed)
		if parsed.Username == "admin" && parsed.Pw == "correct horse" {
			fmt.Fprint(w, `{"AccessToken":"tok-library","User":{"Id":"jf-user-1"}}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("GET /Items", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		seen.gotURL = append(seen.gotURL, r.URL.RawQuery)
		if q.Get("includeItemTypes") != "Movie,Series" || q.Get("recursive") != "true" || q.Get("fields") != "Overview" {
			w.WriteHeader(http.StatusBadRequest)
			var b []byte
			fmt.Fprint(w, `{"Message":"bad query"}`)
			_ = b
			return
		}
		if includeUserID {
			if q.Get("userId") != "jf-user-1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		} else if q.Get("userId") != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var page int
		_, _ = fmt.Sscanf(q.Get("startIndex"), "%d", &page)
		idx := page / 200
		if idx >= len(pages) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"Items":[%s],"TotalRecordCount":%d}`, strings.Join(pages[idx].Items, ","), pages[idx].Count)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, seen
}

func movie(id, name, year string) string {
	return fmt.Sprintf(`{"Id":%q,"Name":%q,"Type":"Movie","ProductionYear":%s}`, id, name, year)
}

func series(id, name string) string {
	return fmt.Sprintf(`{"Id":%q,"Name":%q,"Type":"Series","ProductionYear":0}`, id, name)
}

func TestFetchLibraryPasswordAuth(t *testing.T) {
	srv, seen := newLibraryServer(t, true, libraryPage{Items: []string{movie("jf-1", "Blade Runner", "1982"), series("jf-2", "Severance")}, Count: 2})
	snap, err := NewClient().FetchLibrary(context.Background(), srv.URL, Credentials{Password: &PasswordCredentials{Username: "admin", Password: "correct horse"}})
	if err != nil {
		t.Fatalf("FetchLibrary: %v", err)
	}
	if len(snap.Items) != 2 || snap.Skipped != 0 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.Items[0].Name != "Blade Runner" || snap.Items[0].Kind != "movie" || snap.Items[0].Year != 1982 {
		t.Fatalf("first item = %+v", snap.Items[0])
	}
	if !strings.Contains(seen.gotURL[0], "userId=jf-user-1") {
		t.Fatalf("Items URL missing userId: %s", seen.gotURL[0])
	}
}

func TestFetchLibraryAPIKeyAuth(t *testing.T) {
	srv, seen := newLibraryServer(t, false, libraryPage{Items: []string{movie("jf-1", "Blade Runner", "1982")}, Count: 1})
	snap, err := NewClient().FetchLibrary(context.Background(), srv.URL, Credentials{APIKey: "very-secret-key"})
	if err != nil {
		t.Fatalf("FetchLibrary: %v", err)
	}
	if strings.Contains(seen.gotURL[0], "userId=") {
		t.Fatalf("API-key Items URL must not include userId: %s", seen.gotURL[0])
	}
	if len(snap.Items) != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestFetchLibrarySkipsUnusableItems(t *testing.T) {
	srv, _ := newLibraryServer(t, true, libraryPage{Items: []string{movie("jf-1", "Blade Runner", "1982"), `{"Id":"","Name":"Bad","Type":"Movie","ProductionYear":0}`, `{"Id":"jf-3","Name":"","Type":"Movie","ProductionYear":0}`}, Count: 3})
	snap, err := NewClient().FetchLibrary(context.Background(), srv.URL, Credentials{Password: &PasswordCredentials{Username: "admin", Password: "correct horse"}})
	if err != nil {
		t.Fatalf("FetchLibrary: %v", err)
	}
	if len(snap.Items) != 1 || snap.Skipped != 2 {
		t.Fatalf("snapshot = %+v, want 1 item + 2 skipped", snap)
	}
}

func TestFetchLibraryPaginates(t *testing.T) {
	srv, seen := newLibraryServer(t, true,
		libraryPage{Items: []string{movie("jf-1", "Blade Runner", "1982")}, Count: 2},
		libraryPage{Items: []string{series("jf-2", "Severance")}, Count: 2},
	)
	snap, err := NewClient().FetchLibrary(context.Background(), srv.URL, Credentials{Password: &PasswordCredentials{Username: "admin", Password: "correct horse"}})
	if err != nil {
		t.Fatalf("FetchLibrary: %v", err)
	}
	if len(seen.gotURL) != 2 {
		t.Fatalf("expected 2 pages, got %d: %+v", len(seen.gotURL), seen.gotURL)
	}
	if len(snap.Items) != 2 {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestFetchLibraryEmptyLibrary(t *testing.T) {
	srv, _ := newLibraryServer(t, true, libraryPage{Items: nil, Count: 0})
	snap, err := NewClient().FetchLibrary(context.Background(), srv.URL, Credentials{Password: &PasswordCredentials{Username: "admin", Password: "correct horse"}})
	if err != nil {
		t.Fatalf("FetchLibrary: %v", err)
	}
	if len(snap.Items) != 0 || snap.Skipped != 0 {
		t.Fatalf("snapshot = %+v", snap)
	}
}
