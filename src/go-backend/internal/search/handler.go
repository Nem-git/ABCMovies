package search

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/search/{query}", func(w http.ResponseWriter, r *http.Request) {

		queryStr := r.PathValue("query")

		response := SearchResponse{
			Query: queryStr,
		}

		utils.JSONResponse(w, response)
	})

	return mux
}
