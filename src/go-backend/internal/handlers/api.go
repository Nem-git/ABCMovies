package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/drm"
	"github.com/nem-git/abcmovies/internal/episode"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/recommendations/category"
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
			pis = append(pis, pluginInstance)
		}
	}

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

	// Global endpoints

	mux.Handle("/api/service", service.Handler(pis))

	mux.Handle("/api/stream/", stream.Handler())

	mux.Handle("/api/drm/", drm.Handler())

}
