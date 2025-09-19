package service

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/services", func(w http.ResponseWriter, r *http.Request) {

		response := ServiceResponse{
			Name: "/api/services",
		}

		utils.JSONResponse(w, response)
	})

	mux.HandleFunc("GET /api/services/{serviceTag}", func(w http.ResponseWriter, r *http.Request) {

		request := ServiceRequestHandler(r.PathValue("serviceTag"))

		response := ServiceResponse{
			Id: request.ServiceTag,
		}

		utils.JSONResponse(w, response)
	})

	return mux
}
