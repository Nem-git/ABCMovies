package service

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/service/api"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(pi *plugin.PluginInterface) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/service", func(w http.ResponseWriter, r *http.Request) {

		response := api.ServiceResponse{
			Name: "/api/service",
		}

		utils.JSONResponse(w, response)
	})

	mux.HandleFunc("GET /api/service/{tag}", func(w http.ResponseWriter, r *http.Request) {

		request := api.RequestHandler(r.PathValue("tag"))

		response := api.ServiceResponse{
			Id: request.ServiceTag,
		}

		utils.JSONResponse(w, response)
	})

	return mux
}
