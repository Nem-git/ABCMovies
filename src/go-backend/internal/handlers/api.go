package handlers

import (
	"log"
	"net/http"

	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/requests"
)

func Handler(mux *http.ServeMux) {

	var plugins []*plugin.IPlugin

	availablePlugins := plugin.OpenPlugins()

	for _, p := range availablePlugins {
		pluginInstance, err := plugin.GetInterface("Plugin", p)
		if err == nil {
			log.Println("got instance of plugin:", *p)
			plugins = append(plugins, pluginInstance)
		} else {
			log.Println("error getting instance of plugin:", *p, err)
		}
	}

	// Global endpoints depending on plugins

	// mux.Handle("/api/page", page.Handler(pis))
	// mux.Handle("/api/page/{pageID}", page.Handler(pis))

	// mux.Handle("/api/category", category.Handler(pis))
	// mux.Handle("/api/category/{categoryID}", category.Handler(pis))

	// mux.Handle("/api/search/{query}", search.Handler(pis))

	mux.Handle("/api/service", middleware.PluginsMiddleware(
		middleware.RequestsParsingMiddleware(
			&ServicesHandler{},
			&requests.ServicesRequest{},
		), plugins,
	),
	)

	// Global endpoints

	// mux.Handle("/api/drm/", drm.Handler())

	// mux.Handle("/api/stream/", stream.Handler())

	// // Plugin implementations

	mux.Handle("/api/service/{serviceTag}", middleware.PluginMiddleware(
		middleware.RequestsParsingMiddleware(
			&ServiceHandler{},
			&requests.ServiceRequest{},
		), plugins,
	),
	)

	// mux.Handle("/api/service/{serviceTag}/{showID}", show.Handler(pis))

	// mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}", season.Handler(pis))

	// mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", episode.Handler(pis))
	// mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/next", episode.Handler(pis))
	// mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamID}", episode.Handler(pis))

	// mux.Handle("/api/service/{serviceTag}/search/{query}", search.Handler(pis))

	// mux.Handle("/api/service/{serviceTag}/category", category.Handler(pis))
	// mux.Handle("/api/service/{serviceTag}/category/{categoryID}", category.Handler(pis))
}
