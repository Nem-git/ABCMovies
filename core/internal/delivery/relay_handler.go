package delivery

import (
	"io"
	"net/http"
	"strings"
)

// relayPassthroughHeaders are the provider response headers a relay pull re-
// emits verbatim. A ranged pull returns Content-Range and Content-Length; a
// stream's Content-Type and first-party Accept-Ranges/Cache-Control follow the
// same pipe. The relay never fabricates these — they tell the player exactly
// what the provider answered (PLAN.md §3.6: byte-addressable sinks seek via
// relay URLs).
var relayPassthroughHeaders = []string{
	"Accept-Ranges",
	"Cache-Control",
	"Content-Length",
	"Content-Range",
	"Content-Type",
}

// RelayHandler serves relay tokens as media bytes over HTTP, so a frontend's
// device sink can stream a play session's tracks. Each request resolves the
// token through the Relay, which attaches the provider's auth engine-side; the
// token itself is all a caller ever sees (PLAN.md §3.6).
//
// Tokens are served at /media/relay/{token}, a stable v1 prefix; the serving
// layer mounts this handler wherever its HTTP surface expects media.
type RelayHandler struct {
	Relay *Relay
}

// Handler returns the http.Handler serving relay URLs.
func (h *RelayHandler) Handler() http.Handler { return h }

// ServeHTTP streams the bytes resolved by the token in the request path. A
// caller's Range header is forwarded to the provider, and the provider's own
// status and headers (Content-Range, Content-Length, ...) are re-emitted so
// seeking players get a faithful 206.
func (h *RelayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Relay == nil {
		http.Error(w, "relay not configured", http.StatusServiceUnavailable)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/media/relay/")
	if token == "" || strings.Contains(token, "/") {
		http.Error(w, "missing relay token", http.StatusBadRequest)
		return
	}
	res, err := h.Relay.Open(token, WithRange(r.Header.Get("Range")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	defer func() { _ = res.Body.Close() }()
	hdrs := w.Header()
	for _, k := range relayPassthroughHeaders {
		if v := res.Header.Get(k); v != "" {
			hdrs.Set(k, v)
		}
	}
	if hdrs.Get("Content-Type") == "" {
		hdrs.Set("Content-Type", "application/octet-stream")
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}
