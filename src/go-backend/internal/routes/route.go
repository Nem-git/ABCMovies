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
			api.BadRequestErrorHandler(w, err)
		})
		return
	}

	// Using plugins middleware

	// /service/* routes
	mux.Handle("/api/service", &handlers.ServicesHandler{Plugins: *plugins})
	mux.Handle("/api/service/{serviceTag}", middleware.RequestsParsingMiddleware(&handlers.ServiceHandler{Plugins: *plugins}))
	mux.Handle("/api/service/{serviceTag}/{showID}", middleware.RequestsParsingMiddleware(&handlers.ShowHandler{Plugins: *plugins}))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}", middleware.RequestsParsingMiddleware(&handlers.SeasonHandler{Plugins: *plugins}))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", middleware.RequestsParsingMiddleware(&handlers.EpisodeHandler{Plugins: *plugins}))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/next", middleware.RequestsParsingMiddleware(&handlers.NextEpisodeHandler{Plugins: *plugins}))
	// mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/", middleware.RequestsParsingMiddleware(&handlers.StreamHandler{Plugins: *plugins}))

	// mux.Handle("/api/service/{serviceTag}/category", middleware.PluginMiddleware(RouteCategory(), plugins))
	// mux.Handle("/api/service/{serviceTag}/category/{categoryID}", middleware.PluginMiddleware(RouteCategory(), plugins))

	// // General routes that also need plugins

	mux.Handle("/api/search/{query}", middleware.RequestsParsingMiddleware(&handlers.SearchHandler{Plugins: *plugins}))

	mux.Handle("/api/category", &handlers.CategoriesHandler{Plugins: *plugins})
	mux.Handle("/api/category/{serviceTag}", middleware.RequestsParsingMiddleware(&handlers.ServiceCategoriesHandler{Plugins: *plugins}))
	mux.Handle("/api/category/{serviceTag}/{categoryID}", middleware.RequestsParsingMiddleware(&handlers.CategoryHandler{Plugins: *plugins}))

	// mux.Handle("/api/page", middleware.PluginsMiddleware(RoutePage(), plugins))
	// mux.Handle("/api/page/{pageID}", middleware.PluginsMiddleware(RoutePage(), plugins))

	// // No plugins
	// mux.Handle("/api/drm/", RouteDRM())
	// mux.Handle("/api/stream/", RouteStream())

}
