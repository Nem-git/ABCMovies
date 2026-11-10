package app

import "github.com/Nem-git/ABCMovies/api"

// APIConfig holds configuration for the HTTP API frontend.
type APIConfig = api.APIConfig

// API is a running HTTP API frontend.
type API = api.API

// StartAPI connects to the core over gRPC and serves the HTTP API.
func StartAPI(cfg APIConfig) (*API, error) {
	return api.StartAPI(cfg)
}
