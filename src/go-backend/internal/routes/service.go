package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteService() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/service", middleware.RequestsParsingMiddleware(
		&handlers.ServicesHandler{},
		&requests.ServicesRequest{},
	),
	)

	mux.Handle("/service/{serviceTag}", middleware.RequestsParsingMiddleware(
		&handlers.ServiceHandler{},
		&requests.ServiceRequest{},
	),
	)

	return http.StripPrefix("/api", mux)
}
