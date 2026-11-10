package platform

import (
	"net"
	"net/http"
	"strconv"
	"time"
)

// Listen opens a TCP listener on bind:port and returns its address.
func Listen(bind string, port int) (net.Listener, string, error) {
	addr := net.JoinHostPort(bind, strconv.Itoa(port))
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", err
	}
	return lis, lis.Addr().String(), nil
}

// NewHTTPServer returns a hardened HTTP server with a sane read timeout.
func NewHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
}
