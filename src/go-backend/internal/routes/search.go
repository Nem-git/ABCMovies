package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteSearch() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/{query}", middleware.RequestsParsingMiddleware(
		&handlers.SearchHandler{},
		&requests.SearchRequest{},
	),
	)

	return http.StripPrefix("/api/search", mux)
}
