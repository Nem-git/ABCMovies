package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteDRM() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/drm/{drmID}", middleware.RequestsParsingMiddleware(
		&handlers.StreamHandler{},
		&requests.StreamRequest{},
	),
	)

	return http.StripPrefix("/api", mux)
}
