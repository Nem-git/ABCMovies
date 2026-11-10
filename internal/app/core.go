package app

import "github.com/Nem-git/ABCMovies/core"

// CoreConfig holds configuration for the gRPC core.
type CoreConfig = core.CoreConfig

// Core is a running core (store + gRPC listener). It serves no HTTP.
type Core = core.Core

// StartCore opens the store and serves the identity and source gRPC services.
func StartCore(cfg CoreConfig) (*Core, error) {
	return core.StartCore(cfg)
}
