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
	tok, err := relay.Grant("s1", "v1", func(ctx context.Context) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("stream-bytes")), nil
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
