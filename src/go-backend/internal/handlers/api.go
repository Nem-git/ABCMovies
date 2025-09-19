package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/drm/widevine"
	"github.com/nem-git/abcmovies/internal/episode"
	"github.com/nem-git/abcmovies/internal/season"
	"github.com/nem-git/abcmovies/internal/services"
	"github.com/nem-git/abcmovies/internal/show"
	"github.com/nem-git/abcmovies/internal/stream/dash"
)

func Handler(mux *http.ServeMux) {

	mux.Handle("/api/services", services.Handler())
	mux.Handle("/api/services/{serviceTag}", services.Handler())
	mux.Handle("/api/services/{serviceTag}/{showID}", show.Handler())
	mux.Handle("/api/services/{serviceTag}/{showID}/{seasonNumber}", season.Handler())
	mux.Handle("/api/services/{serviceTag}/{showID}/{seasonNumber}/{episodeNumber}", episode.Handler())

	mux.Handle("/api/stream/dash/", dash.Handler())
	mux.Handle("/api/drm/widevine/", widevine.Handler())

}
