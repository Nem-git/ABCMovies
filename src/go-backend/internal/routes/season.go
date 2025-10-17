package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteSeason() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/{serviceTag}/{showID}/{seasonNumber}", middleware.RequestsParsingMiddleware(
		&handlers.SeasonHandler{},
		&requests.SeasonRequest{},
	),
	)

	return http.StripPrefix("/api/service", mux)
}
