package category

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/plugin"
)

func Handler(pis []*plugin.PluginInterface) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /categories", func(w http.ResponseWriter, r *http.Request) {

		// content, err := GetManifest(model.Url, model.Content)
		// if err != nil {
		// 	api.InternalErrorHandler(w, err)
		// 	return
		// }

		// response := DashManifestResponse{
		// 	Content: content,
		// }

		// utils.JSONResponse(w, response)
	})

	return http.StripPrefix("/api/categories", mux)
}
