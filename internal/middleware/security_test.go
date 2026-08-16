package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadAsAttachment(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := DownloadAsAttachment(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1alpha/services/T/movies/m1/streams/mp4", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="video.mp4"` {
		t.Errorf("Content-Disposition = %q, want attachment", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1alpha/services/T/movies/m1/streams/hls", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("Content-Disposition set for non-mp4 route: %q", got)
	}
}
