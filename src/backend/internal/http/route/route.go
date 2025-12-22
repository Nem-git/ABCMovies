package route

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/http/api"
	"github.com/nem-git/abcmovies/internal/http/handler"
	"github.com/nem-git/abcmovies/internal/http/middleware"
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
	mux.Handle("/api/service", handler.NewServicesHandler(plugins))
	mux.Handle("/api/service/{serviceTag}", middleware.RequestsParsingMiddleware(handler.NewServiceHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}", middleware.RequestsParsingMiddleware(handler.NewShowHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}", middleware.RequestsParsingMiddleware(handler.NewSeasonHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", middleware.RequestsParsingMiddleware(handler.NewEpisodeHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/next", middleware.RequestsParsingMiddleware(handler.NewNextEpisodeHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/{streamFileName}", middleware.RequestsParsingMiddleware(handler.NewStreamHandler(plugins)))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/{streamURL...}", middleware.RequestsParsingMiddleware(handler.NewStreamHandler(plugins)))

	// Dash
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/{streamMediaType}/{streamID}/{streamURL...}", middleware.RequestsParsingMiddleware(handler.NewStreamHandler(plugins)))

	// // General routes that also need plugins

	mux.Handle("/api/search/{query}", middleware.RequestsParsingMiddleware(handler.NewSearchHandler(plugins)))
	mux.Handle("/api/search/{serviceTag}/{query}", middleware.RequestsParsingMiddleware(handler.NewServiceSearchHandler(plugins)))

	mux.Handle("/api/category", handler.NewCategoriesHandler(plugins))
	mux.Handle("/api/category/{serviceTag}", middleware.RequestsParsingMiddleware(handler.NewServiceCategoryHandler(plugins)))
	mux.Handle("/api/category/{serviceTag}/{categoryID}", middleware.RequestsParsingMiddleware(handler.NewCategoryHandler(plugins)))

	// mux.Handle("/api/page", middleware.PluginsMiddleware(RoutePage(), plugins))
	// mux.Handle("/api/page/{pageID}", middleware.PluginsMiddleware(RoutePage(), plugins))

	// // No plugins
	// mux.Handle("/api/drm/", RouteDRM())
	// mux.Handle("/api/stream/", RouteStream())

}
