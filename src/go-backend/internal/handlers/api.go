package handlers

import (
	"errors"
	"net/http"

	"github.com/nem-git/abcmovies/api"
	"github.com/nem-git/abcmovies/internal/drm/widevine"
	"github.com/nem-git/abcmovies/internal/streaming/dash"
)

func Handler(mux *http.ServeMux) {

	mux.Handle("/api/stream/dash/", dash.Handler())
	mux.Handle("/api/drm/widevine/", widevine.Handler())

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		api.BadRequestErrorHandler(w, errors.New("endpoint does not exist"))
	})
}
