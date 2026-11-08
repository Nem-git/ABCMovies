// Package core holds core runtime helpers for ABCMovies.
package core

// Live reports the core is alive. It exists so the standard check command
// has a meaningful smoke assertion on a fresh checkout.
func Live() string { return "ok" }