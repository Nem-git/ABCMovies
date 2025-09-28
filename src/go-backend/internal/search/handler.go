package search

import (
	"net/http"

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
			res := &searchApi.SearchResponse{}
			_ = (*pi).GetSearch(request, response) // err
			response.Shows = append(response.Shows, res.Shows...)
		}

		response.Query = query
		response.ShowCount = len(response.Shows)

		utils.JSONResponse(w, *response)
	})

	return mux
}
