package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteEpisode() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", middleware.RequestsParsingMiddleware(
		&handlers.EpisodeHandler{},
		&requests.EpisodeRequest{},
	),
	)

	return http.StripPrefix("/api", mux)
}
