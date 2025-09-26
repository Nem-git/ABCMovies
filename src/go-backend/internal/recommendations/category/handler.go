package category

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/recommendations/category/api"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(pis []*plugin.PluginInterface) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/category", func(w http.ResponseWriter, r *http.Request) {

		request := api.CategoriesRequest{}
		response := &api.CategoriesResponse{}

		for _, pi := range pis {
			res := &api.CategoriesResponse{}
			(*pi).GetCategories(request, res)
			response.Categories = append(response.Categories, res.Categories...)
		}

		response.CategoriesCount = len(response.Categories)

		utils.JSONResponse(w, *response)
	})

	mux.HandleFunc("GET /api/category/{categoryID}", func(w http.ResponseWriter, r *http.Request) {

		categoryID := r.PathValue("categoryID")

		request := api.CategoryRequestHandler(categoryID)
		response := &api.CategoryResponse{}

		for _, pi := range pis {
			res := &api.CategoryResponse{}
			(*pi).GetCategory(request, res)
			response.Shows = append(response.Shows, res.Shows...)
		}

		utils.JSONResponse(w, *response)
	})

	return mux
}
