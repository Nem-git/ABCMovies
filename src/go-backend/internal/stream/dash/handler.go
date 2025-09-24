package dash

import (
	"fmt"
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	dashApi "github.com/nem-git/abcmovies/internal/stream/dash/api"
	"github.com/nem-git/abcmovies/internal/stream/dash/manifest"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("POST /manifest", func(w http.ResponseWriter, r *http.Request) {

		model := &dashApi.DashManifestRequest{}

		if err := utils.BindJSON(r, model); err != nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("invalid json body"))
			return
		}

		if model.URL == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: url"))
			return
		}
		if model.Content == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: content"))
			return
		}

		content, err := manifest.Get(model.URL, model.Content)
		if err != nil {
			api.InternalErrorHandler(w, err)
			return
		}

		response := dashApi.DashManifestResponse{
			Content: content,
		}

		utils.JSONResponse(w, response)
	})

	return http.StripPrefix("/api/stream/dash", mux)
}
