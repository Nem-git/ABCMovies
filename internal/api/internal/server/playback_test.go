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

func TestStreamProxyRangeAndNotFound(t *testing.T) {
	payload := []byte("0123456789")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Accept-Ranges", "bytes")
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
