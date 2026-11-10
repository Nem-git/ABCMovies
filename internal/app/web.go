package app

import "github.com/Nem-git/ABCMovies/web"

// WebConfig holds configuration for the SSR web frontend.
type WebConfig = web.WebConfig

// Web is a running SSR web frontend.
type Web = web.Web

// StartWeb serves the SSR web frontend against the given HTTP API.
func StartWeb(cfg WebConfig) (*Web, error) {
	return web.StartWeb(cfg)
}
