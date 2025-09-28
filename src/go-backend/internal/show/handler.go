package show

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/plugin"
	showApi "github.com/nem-git/abcmovies/internal/show/api"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(pis []*plugin.PluginInterface) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/service/{serviceTag}/{showID}", func(w http.ResponseWriter, r *http.Request) {

		tag := r.PathValue("serviceTag")
		showID := r.PathValue("showID")

		pi, err := utils.GetPluginBySlug(tag, pis)
		if err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		request := showApi.ShowRequestHandler(tag, showID)

		response := &showApi.ShowResponse{}

		if err := (*pi).GetShow(request, response); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		utils.JSONResponse(w, *response)
	})

	return mux
}
