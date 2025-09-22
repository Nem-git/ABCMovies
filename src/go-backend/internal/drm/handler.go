package stream

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/drm/widevine"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.Handle("GET /api/drm/widevine/", widevine.Handler())
	// mux.Handle("GET /api/drm/hls/", hls.Handler())
	// mux.Handle("GET /api/drm/smooth/", smooth.Handler())

	return mux
}
