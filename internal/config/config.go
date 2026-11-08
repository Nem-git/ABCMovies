// Package config provides tiny helpers for reading environment configuration
// for the ABCMovies binaries. Config values are names only in the repo; real
// values come from the environment at runtime.
package config

import (
	"os"
	"strconv"
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
