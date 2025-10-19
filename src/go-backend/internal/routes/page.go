package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RoutePage() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/page", &handlers.PagesHandler{})

	mux.Handle("/page/{pageID}", middleware.RequestsParsingMiddleware(
		&handlers.PageHandler{},
		&requests.PageRequest{},
	),
	)

	return http.StripPrefix("/api", mux)
}
