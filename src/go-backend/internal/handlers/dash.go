package handlers

import (
	"fmt"
	"net/http"

	"github.com/nem-git/abcmovies/api"
	"github.com/nem-git/abcmovies/internal/streaming"
	"github.com/nem-git/abcmovies/internal/utils"
)

func DashHandler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("POST /manifest", func(w http.ResponseWriter, r *http.Request) {

		model := &api.DashManifestRequest{}

		if err := utils.BindJSON(r, model); err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("invalid json body"))
			return
		}

		if model.Url == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: url"))
			return
		}
		if model.Content == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: content"))
			return
		}

		content, err := streaming.GetManifest(model.Url, model.Content)
		if err != nil {
			api.InternalErrorHandler(w, err)
			return
		}

		response := api.DashManifestResponse{
			Content: content,
		}

		utils.JSONResponse(w, response)
	})

	return http.StripPrefix("/api/stream/dash", mux)
}
