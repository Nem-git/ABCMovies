// Package service holds the ABCMovies core gRPC services.
package service

// Live reports the core is alive. It exists so the standard check command
// has a meaningful smoke assertion on a fresh checkout.
func Live() string { return "ok" }
