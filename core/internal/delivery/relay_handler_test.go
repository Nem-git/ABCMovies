package delivery

import (
	"context"
	"errors"
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
	req := httptest.NewRequest(http.MethodGet, RelayPrefix+tok, nil)
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
	req := httptest.NewRequest(http.MethodGet, RelayPrefix+tok, nil)
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

// A pull runs under the caller's own context, so a caller that goes away —
// a closed browser tab, a seeking player that gave up — cancels the provider
// fetch instead of leaving it running to completion against the provider's
// goodwill. The fetch closure is handed the same ctx the request carried.
func TestRelayPullIsCancelledWithTheCaller(t *testing.T) {
	relay := NewRelay()
	fetched := make(chan struct{})
	tok, err := relay.Grant("s1", "v1", func(ctx context.Context, _ string) (FetchResult, error) {
		close(fetched)
		<-ctx.Done() // the provider fetch is in flight when the caller leaves
		return FetchResult{}, ctx.Err()
	}, time.Minute)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := relay.Open(ctx, tok)
		done <- err
	}()

	<-fetched // the fetch has started under the caller's context
	cancel()  // the caller disconnects

	// Bounded: if the pull ignores the caller's context this fails fast
	// rather than hanging the suite.
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("pull err = %v, want context.Canceled (the caller's own context)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pull still running after the caller's context was cancelled: Open is not honouring it")
	}
}

// The request handler forwards exactly that context: ServeHTTP's own
// disconnect path must reach the fetch, not a fresh background one.
func TestRelayHandlerForwardsRequestContext(t *testing.T) {
	relay := NewRelay()
	fetched := make(chan struct{})
	tok, err := relay.Grant("s1", "v1", func(ctx context.Context, _ string) (FetchResult, error) {
		close(fetched)
		<-ctx.Done()
		return FetchResult{}, ctx.Err()
	}, time.Minute)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}

	reqCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, RelayPrefix+tok, nil).WithContext(reqCtx)
	done := make(chan int, 1)
	go func() {
		rr := httptest.NewRecorder()
		(&RelayHandler{Relay: relay}).ServeHTTP(rr, req)
		done <- rr.Code
	}()

	<-fetched
	cancel()

	// Bounded: a handler that dropped the request context would block here.
	select {
	case code := <-done:
		if code != http.StatusForbidden {
			t.Fatalf("status after caller cancelled = %d, want 403 (the fetch reported the cancellation)", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handler still running after the request context was cancelled: it is not forwarding it")
	}
}

func TestRelayHandlerRejectsUnknownAndShapedPaths(t *testing.T) {
	relay := NewRelay()
	h := &RelayHandler{Relay: relay}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, RelayPrefix+"bogus", nil))
	if rr.Code != http.StatusForbidden {
		t.Errorf("unknown token status = %d, want 403", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, RelayPrefix, nil))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("empty token status = %d, want 400", rr.Code)
	}
}
