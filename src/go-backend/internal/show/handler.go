package show

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/services/{serviceTag}/{showID}", func(w http.ResponseWriter, r *http.Request) {

		request := ShowRequestHandler(r.PathValue("serviceTag"), r.PathValue("showID"))

		response := ShowResponse{
			Name: request.ServiceTag,
			ID:   request.ShowID,
		}

		utils.JSONResponse(w, response)
	})

	return mux
}
