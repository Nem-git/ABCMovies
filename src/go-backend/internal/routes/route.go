package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/handlers"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/plugin"
)

func Handler(mux *http.ServeMux) {

	plugins, err := plugin.Load()
	if err != nil {
		mux.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
			api.InternalErrorHandler(w, err)
		})
		return
	}

	// Using plugins middleware

	// /service/* routes
	mux.Handle("/api/service", handlers.NewServicesHandler(plugins))
	mux.Handle("/api/service/{serviceTag}", middleware.RequestsParsingMiddleware(handlers.NewServiceHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}", middleware.RequestsParsingMiddleware(handlers.NewShowHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}", middleware.RequestsParsingMiddleware(handlers.NewSeasonHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", middleware.RequestsParsingMiddleware(handlers.NewEpisodeHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/next", middleware.RequestsParsingMiddleware(handlers.NewNextEpisodeHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/{streamFileName}", middleware.RequestsParsingMiddleware(handlers.NewStreamHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/{streamURL...}", middleware.RequestsParsingMiddleware(handlers.NewStreamHandler(plugins)))

	// Dash
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/{streamMediaType}/{streamID}/{streamURL...}", middleware.RequestsParsingMiddleware(handlers.NewStreamHandler(plugins)))

	// // General routes that also need plugins

	mux.Handle("/api/search/{query}", middleware.RequestsParsingMiddleware(handlers.NewSearchHandler(plugins)))
	mux.Handle("/api/search/{serviceTag}/{query}", middleware.RequestsParsingMiddleware(handlers.NewServiceSearchHandler(plugins)))

	mux.Handle("/api/category", handlers.NewCategoriesHandler(plugins))
	mux.Handle("/api/category/{serviceTag}", middleware.RequestsParsingMiddleware(handlers.NewServiceCategoryHandler(plugins)))
	mux.Handle("/api/category/{serviceTag}/{categoryID}", middleware.RequestsParsingMiddleware(handlers.NewCategoryHandler(plugins)))

	// mux.Handle("/api/page", middleware.PluginsMiddleware(RoutePage(), plugins))
	// mux.Handle("/api/page/{pageID}", middleware.PluginsMiddleware(RoutePage(), plugins))

	// // No plugins
	// mux.Handle("/api/drm/", RouteDRM())
	// mux.Handle("/api/stream/", RouteStream())

}
