package service

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/service", func(w http.ResponseWriter, r *http.Request) {

		response := ServiceResponse{
			Name: "/api/service",
		}

		utils.JSONResponse(w, response)
	})

	mux.HandleFunc("GET /api/service/{tag}", func(w http.ResponseWriter, r *http.Request) {

		request := RequestHandler(r.PathValue("tag"))

		response := ServiceResponse{
			Id: request.ServiceTag,
		}

		utils.JSONResponse(w, response)
	})

	return mux
}
