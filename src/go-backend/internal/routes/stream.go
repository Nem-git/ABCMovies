package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteStream() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/{streamFileName}", middleware.RequestsParsingMiddleware(
		&handlers.StreamHandler{},
		&requests.StreamRequest{},
	),
	)

	mux.Handle("/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/{streamURL...}", middleware.RequestsParsingMiddleware(
		&handlers.StreamHandler{},
		&requests.StreamRequest{},
	),
	)

	// Dash
	mux.Handle("/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/{streamMediaType}/{streamID}/{streamURL...}", middleware.RequestsParsingMiddleware(
		&handlers.StreamHandler{},
		&requests.StreamRequest{},
	),
	)

	return http.StripPrefix("/api/service", mux)
}
