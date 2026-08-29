package delivery

import (
	"io"
	"net/http"
	"strings"
)

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

// ServeHTTP streams the bytes resolved by the token in the request path.
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
	body, _, err := h.Relay.Open(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	defer func() { _ = body.Close() }()
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, body)
}
