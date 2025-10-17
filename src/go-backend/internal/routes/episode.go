package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteEpisode() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", middleware.RequestsParsingMiddleware(
		&handlers.EpisodeHandler{},
		&requests.EpisodeRequest{},
	),
	)

	mux.Handle("/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/next", middleware.RequestsParsingMiddleware(
		&handlers.NextEpisodeHandler{},
		&requests.EpisodeRequest{},
	),
	)

	return http.StripPrefix("/api/service", mux)
}
