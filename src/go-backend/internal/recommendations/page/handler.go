package page

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/recommendations/page/api"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(pis []*plugin.PluginInterface) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/page", func(w http.ResponseWriter, r *http.Request) {

		_ = &api.PagesRequest{}
		var response api.PagesResponse

		// for _, pi := range pis {
		// 	res := &api.PageResponse{}
		// 	response = append(response, res)
		// }

		utils.JSONResponse(w, response)
	})

	mux.HandleFunc("GET /api/page/{pageID}", func(w http.ResponseWriter, r *http.Request) {

		pageID := r.PathValue("pageID")

		_ = api.PageRequestHandler(pageID)
		response := &api.PageResponse{}

		// for _, pi := range pis {
		// 	res := &api.PageResponse{}
		// 	response = append(response, res)
		// }

		utils.JSONResponse(w, *response)
	})

	return mux
}
