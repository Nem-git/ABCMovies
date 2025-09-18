package handlers

import (
	"errors"
	"net/http"

	"github.com/nem-git/abcmovies/api"
)

func Handler(mux *http.ServeMux) {

	mux.Handle("/api/stream/dash/", DashHandler())
	mux.Handle("/api/drm/widevine/", WidevineHandler())

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		api.BadRequestErrorHandler(w, errors.New("endpoint does not exist"))
	})
}
