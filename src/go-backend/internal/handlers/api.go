package handlers

import (
	"log"
	"net/http"

	"github.com/nem-git/abcmovies/internal/drm"
	"github.com/nem-git/abcmovies/internal/episode"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/recommendations/category"
	"github.com/nem-git/abcmovies/internal/recommendations/page"
	"github.com/nem-git/abcmovies/internal/search"
	"github.com/nem-git/abcmovies/internal/season"
	"github.com/nem-git/abcmovies/internal/service"
	"github.com/nem-git/abcmovies/internal/show"
	"github.com/nem-git/abcmovies/internal/stream"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(mux *http.ServeMux) {

	var pis []*plugin.PluginInterface

	availablePlugins := utils.OpenPlugins()

	for _, p := range availablePlugins {
		pluginInstance, err := utils.GetPluginInterface("Plugin", p)
		if err == nil {
			log.Println("got instance of plugin:", *p)
			pis = append(pis, pluginInstance)
		} else {
			log.Println("error getting instance of plugin:", *p, err)
		}
	}

	// Global endpoints depending on plugins

	mux.Handle("/api/page", page.Handler(pis))
	mux.Handle("/api/page/{pageID}", page.Handler(pis))

	mux.Handle("/api/category", category.Handler(pis))
	mux.Handle("/api/category/{categoryID}", category.Handler(pis))

	mux.Handle("/api/search/{query}", search.Handler(pis))

	mux.Handle("/api/service", service.Handler(pis))

	// Global endpoints

	mux.Handle("/api/drm/", drm.Handler())

	mux.Handle("/api/stream/", stream.Handler())

	// Plugin implementations

	mux.Handle("/api/service/{serviceTag}", service.Handler(pis))

	mux.Handle("/api/service/{serviceTag}/{showID}", show.Handler(pis))

	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}", season.Handler(pis))

	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", episode.Handler(pis))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/next", episode.Handler(pis))
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamID}", episode.Handler(pis))

	mux.Handle("/api/service/{serviceTag}/search/{query}", search.Handler(pis))

	mux.Handle("/api/service/{serviceTag}/category", category.Handler(pis))
	mux.Handle("/api/service/{serviceTag}/category/{categoryID}", category.Handler(pis))
}
