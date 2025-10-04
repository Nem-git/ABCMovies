package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteCategory() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/service/{serviceTag}/category", middleware.RequestsParsingMiddleware(
		&handlers.CategoriesHandler{},
		&requests.CategoriesRequest{},
	),
	)

	mux.Handle("/service/{serviceTag}/category/{categoryID}", middleware.RequestsParsingMiddleware(
		&handlers.CategoryHandler{},
		&requests.CategoryRequest{},
	),
	)

	return http.StripPrefix("/api", mux)
}
