package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteShow() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/service/{serviceTag}/{showID}", middleware.RequestsParsingMiddleware(
		&handlers.ShowHandler{},
		&requests.ShowRequest{},
	),
	)

	return http.StripPrefix("/api", mux)
}
