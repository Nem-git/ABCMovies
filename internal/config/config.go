// Package config provides tiny helpers for reading environment configuration
// for the ABCMovies binaries. Config values are names only in the repo; real
// values come from the environment at runtime.
package config

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Env returns the value of key, or def if unset or empty.
func Env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// EnvPort returns the value of key parsed as an int, or def if unset, empty,
// or not a valid port number.
func EnvPort(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 65535 {
			return n
		}
	}
	return def
}

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
