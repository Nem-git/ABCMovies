package search

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/plugin"
	searchApi "github.com/nem-git/abcmovies/internal/search/api"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(pis []*plugin.PluginInterface) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/search/{query}", func(w http.ResponseWriter, r *http.Request) {

		query := r.PathValue("query")

		request := searchApi.RequestHandler(query)
		response := &searchApi.SearchResponse{}

		for _, pi := range pis {
			(*pi).GetSearch(request, response)
		}

		utils.JSONResponse(w, *response)
	})

	mux.HandleFunc("GET /api/service/{serviceTag}/search/{query}", func(w http.ResponseWriter, r *http.Request) {

		tag := r.PathValue("serviceTag")
		query := r.PathValue("query")

		pi, err := utils.GetPluginBySlug(tag, pis)
		if err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		request := searchApi.RequestHandler(query)

		response := &searchApi.SearchResponse{}

		(*pi).GetSearch(request, response)

		utils.JSONResponse(w, *response)
	})

	return mux
}
