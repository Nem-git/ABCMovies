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
	serviceApi "github.com/nem-git/abcmovies/internal/service/api"
	"github.com/nem-git/abcmovies/internal/show"
	"github.com/nem-git/abcmovies/internal/stream"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(mux *http.ServeMux) {

	var plugins []*plugin.PluginInterface

	availablePlugins := utils.OpenPlugins()

	for _, p := range availablePlugins {
		pluginInstance, err := utils.GetPluginInterface("Plugin", p)
		if err == nil {
			plugins = append(plugins, pluginInstance)
		}
	}

	// Plugin implementations
	mux.Handle("/api/service/{serviceTag}", service.Handler())

	mux.Handle("/api/service/{serviceTag}/{showID}", show.Handler())

	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}", season.Handler())

	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", episode.Handler())
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/next", episode.Handler())
	mux.Handle("/api/service/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}/{streamID}", episode.Handler())

	mux.Handle("/api/service/{serviceTag}/search/{query}", search.Handler())

	mux.Handle("/api/service/{serviceTag}/category", category.Handler())
	mux.Handle("/api/service/{serviceTag}/category/{categoryID}", category.Handler())

	// Global endpoints

	mux.Handle("/api/service", service.Handler())

	mux.Handle("/api/stream/", stream.Handler())

	mux.Handle("/api/drm/", drm.Handler())

}
