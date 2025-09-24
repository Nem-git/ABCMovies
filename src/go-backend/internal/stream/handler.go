package stream

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/stream/dash"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.Handle("GET /api/stream/dash/", dash.Handler())
	// mux.Handle("GET /api/stream/hls/", hls.Handler())
	// mux.Handle("GET /api/stream/smooth/", smooth.Handler())

	return mux
}
