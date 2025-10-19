package routes

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/plugin"
)

func Handler(mux *http.ServeMux) {

	var plugins []*plugin.IPlugin

	if err := plugin.Load(&plugins); err != nil {
		mux.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
			api.BadRequestErrorHandler(w, err)
		})
		return
	}

	// Using plugins middleware

	// /service/* routes
	mux.Handle("/api/service", middleware.PluginsMiddleware(RouteService(), plugins))
	mux.Handle("/api/service/{serviceTag}", middleware.PluginMiddleware(RouteService(), plugins))
	mux.Handle("/api/service/{serviceTag}/{showID}", middleware.PluginMiddleware(RouteShow(), plugins))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}", middleware.PluginMiddleware(RouteSeason(), plugins))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", middleware.PluginMiddleware(RouteEpisode(), plugins))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/next", middleware.PluginMiddleware(RouteEpisode(), plugins))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamType}/", middleware.PluginMiddleware(RouteStream(), plugins))

	mux.Handle("/api/service/{serviceTag}/category", middleware.PluginMiddleware(RouteCategory(), plugins))
	mux.Handle("/api/service/{serviceTag}/category/{categoryID}", middleware.PluginMiddleware(RouteCategory(), plugins))

	// General routes that also need plugins

	mux.Handle("/api/search/", middleware.PluginsMiddleware(RouteSearch(), plugins))

	mux.Handle("/api/category", middleware.PluginsMiddleware(RouteCategory(), plugins))

	mux.Handle("/api/page", middleware.PluginsMiddleware(RoutePage(), plugins))
	mux.Handle("/api/page/{pageID}", middleware.PluginsMiddleware(RoutePage(), plugins))

	// No plugins
	mux.Handle("/api/drm/", RouteDRM())
	mux.Handle("/api/stream/", RouteStream())

}
