package delivery

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRelayHandlerServesMediaOverHTTP(t *testing.T) {
	relay := NewRelay()
	tok, err := relay.Grant("s1", "v1", func(ctx context.Context, _ string) (FetchResult, error) {
		return FetchResult{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("stream-bytes")),
		}, nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/media/relay/"+tok, nil)
	(&RelayHandler{Relay: relay}).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "stream-bytes" {
		t.Errorf("body = %q", rr.Body.String())
	}
}

// TestRelayHandlerForwardsRange proves a ranged pull reaches the provider with
// the Range header and the provider's 206 partial response — status,
// Content-Range, and bytes — is what the caller receives.
func TestRelayHandlerForwardsRange(t *testing.T) {
	relay := NewRelay()
	tok, err := relay.Grant("s1", "v1", func(_ context.Context, rangeHeader string) (FetchResult, error) {
		if rangeHeader != "bytes=0-3" {
			return FetchResult{}, &rangeError{got: rangeHeader}
		}
		return FetchResult{
			StatusCode: http.StatusPartialContent,
			Header: http.Header{
				"Content-Range": {"bytes 0-3/11"},
				"Content-Type":  {"video/mp4"},
			},
			Body: io.NopCloser(strings.NewReader("stre")),
		}, nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/media/relay/"+tok, nil)
	req.Header.Set("Range", "bytes=0-3")
	(&RelayHandler{Relay: relay}).ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206; body=%q", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Range") != "bytes 0-3/11" {
		t.Errorf("Content-Range = %q", rr.Header().Get("Content-Range"))
	}
	if rr.Header().Get("Content-Type") != "video/mp4" {
		t.Errorf("Content-Type = %q", rr.Header().Get("Content-Type"))
	}
	if rr.Body.String() != "stre" {
		t.Errorf("body = %q, want the ranged bytes", rr.Body.String())
	}
}

type rangeError struct{ got string }

func (e *rangeError) Error() string { return "range not forwarded: " + e.got }

func TestRelayHandlerRejectsUnknownAndShapedPaths(t *testing.T) {
	relay := NewRelay()
	h := &RelayHandler{Relay: relay}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/media/relay/bogus", nil))
	if rr.Code != http.StatusForbidden {
		t.Errorf("unknown token status = %d, want 403", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/media/relay/", nil))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("empty token status = %d, want 400", rr.Code)
	}
}
