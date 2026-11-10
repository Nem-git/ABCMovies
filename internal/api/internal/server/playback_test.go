package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
)

func TestPlayTicketHappyPath(t *testing.T) {
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		createPlayTicket: func(_ context.Context, in *libraryv1.CreatePlayTicketRequest, _ ...grpc.CallOption) (*libraryv1.CreatePlayTicketResponse, error) {
			if got := in.GetActorUserId(); got != "usr_admin" {
				t.Errorf("actor = %q, want usr_admin", got)
			}
			if in.GetPassword() != nil || in.GetApiKey() != "" {
				t.Errorf("expected no credentials oneof (session cache path)")
			}
			return &libraryv1.CreatePlayTicketResponse{Ticket: "tick_abc"}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/library/title/ttl_1/play", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "abcmovies_session=sess_1")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("play status = %d, body=%s", rr.Code, rr.Body)
	}
	var out playTicketResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Ticket != "tick_abc" {
		t.Fatalf("ticket = %q, want tick_abc", out.Ticket)
	}
	if !strings.HasPrefix(out.StreamHref, "/api/library/title/ttl_1/stream?ticket=") {
		t.Fatalf("stream_href = %q", out.StreamHref)
	}
}

func TestStreamProxyRedirectsAndUpstreamServerError(t *testing.T) {
	// secret stands in for anything the source might try to make the server
	// fetch elsewhere on the network (SSRF).
	secret := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("internal-secret-bytes"))
	}))
	defer secret.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect-foreign":
			w.Header().Set("Location", secret.URL+"/")
			w.WriteHeader(http.StatusFound)
		case "/redirect-self":
			w.Header().Set("Location", "/file")
			w.WriteHeader(http.StatusFound)
		case "/boom":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("upstream-oops-html-body"))
		default:
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("media-bytes"))
		}
	}))
	defer source.Close()

	srv := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		resolvePlayTicket: func(_ context.Context, in *libraryv1.ResolvePlayTicketRequest, _ ...grpc.CallOption) (*libraryv1.ResolvePlayTicketResponse, error) {
		url := source.URL
		switch in.GetTicket() {
		case "foreign":
			url += "/redirect-foreign"
		case "self":
			url += "/redirect-self"
		case "boom":
			url += "/boom"
		}
		return &libraryv1.ResolvePlayTicketResponse{TitleId: "ttl_1", SourceId: "src_1", StreamUrl: url, Authorization: `MediaBrowser Token="t"`}, nil
	},
	})
	do := func(ticket string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/library/title/ttl_1/stream?ticket="+ticket, nil)
		req.Header.Set("Cookie", "abcmovies_session=sess_1")
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		return rr
	}

	// Cross-origin redirect: refused, 503, secret bytes never relayed.
	rr := do("foreign")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("foreign redirect status = %d, want 503", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "internal-secret-bytes") {
		t.Fatal("the proxy relayed bytes from a cross-origin redirect")
	}

	// Same-origin redirect: followed.
	rr = do("self")
	if rr.Code != http.StatusOK || rr.Body.String() != "media-bytes" {
		t.Fatalf("self redirect = %d %q, want 200 media-bytes", rr.Code, rr.Body.String())
	}

	// Upstream 5xx: 502, and the upstream body is not relayed verbatim.
	rr = do("boom")
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("upstream 500 passthrough status = %d, want 502", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "upstream-oops-html-body") {
		t.Fatal("upstream error body was relayed to the browser")
	}
}

func TestStreamProxyRangeAndNotFound(t *testing.T) {
	payload := []byte("0123456789")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"upstream-etag"`)
		w.Header().Set("Last-Modified", "Mon, 01 Jan 2024 00:00:00 GMT")
		if rng := r.Header.Get("Range"); strings.HasPrefix(rng, "bytes=") {
			spec := strings.TrimPrefix(rng, "bytes=")
			var start, end int
			if _, err := fmt.Sscanf(spec, "%d-%d", &start, &end); err != nil {
				end = len(payload) - 1
				if _, err := fmt.Sscanf(spec, "%d-", &start); err != nil {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = fmt.Fprint(w, string(payload[start:end+1]))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer upstream.Close()

	srv := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		resolvePlayTicket: func(_ context.Context, in *libraryv1.ResolvePlayTicketRequest, _ ...grpc.CallOption) (*libraryv1.ResolvePlayTicketResponse, error) {
			if in.GetTicket() == "bad" {
				return nil, status.Error(codes.NotFound, "not found")
			}
			return &libraryv1.ResolvePlayTicketResponse{
				TitleId:       "ttl_1",
				SourceId:      "src_1",
				StreamUrl:     upstream.URL + "/Videos/jf-1/stream?static=true",
				Authorization: `MediaBrowser Token="tok"`,
			}, nil
		},
	})

	do := func(method, target, cookie, rng string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		if rng != "" {
			req.Header.Set("Range", rng)
		}
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		return rr
	}

	// Full GET.
	rr := do(http.MethodGet, "/api/library/title/ttl_1/stream?ticket=good", "abcmovies_session=sess_1", "")
	if rr.Code != http.StatusOK || rr.Body.String() != string(payload) {
		t.Fatalf("stream = %d %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("ETag"); got != "" {
		t.Fatalf("ETag forwarded from upstream = %q; want it dropped (no-store)", got)
	}
	if got := rr.Header().Get("Last-Modified"); got != "" {
		t.Fatalf("Last-Modified forwarded from upstream = %q; want it dropped (no-store)", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}

	// Range passthrough.
	rr = do(http.MethodGet, "/api/library/title/ttl_1/stream?ticket=good", "abcmovies_session=sess_1", "bytes=2-5")
	if rr.Code != http.StatusPartialContent || rr.Body.String() != "2345" {
		t.Fatalf("range stream = %d %q", rr.Code, rr.Body.String())
	}

	// Unknown ticket: 404.
	rr = do(http.MethodGet, "/api/library/title/ttl_1/stream?ticket=bad", "abcmovies_session=sess_1", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("bad ticket = %d, want 404", rr.Code)
	}

	// No session: 401.
	rr = do(http.MethodGet, "/api/library/title/ttl_1/stream?ticket=good", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous stream = %d, want 401", rr.Code)
	}
}
