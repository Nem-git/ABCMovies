package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/requests"
)

func RouteService() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/service", &handlers.ServicesHandler{})

	mux.Handle("/service/{serviceTag}", middleware.RequestsParsingMiddleware(
		&handlers.ServiceHandler{},
		&requests.ServiceRequest{},
	),
	)

	return http.StripPrefix("/api", mux)
}
