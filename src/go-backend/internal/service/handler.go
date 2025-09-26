package service

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/plugin"
	serviceApi "github.com/nem-git/abcmovies/internal/service/api"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler(pis []*plugin.PluginInterface) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/service", func(w http.ResponseWriter, r *http.Request) {

		var services []serviceApi.ServiceResponse

		for _, pi := range pis {
			req := serviceApi.ServiceRequest{}
			res := &serviceApi.ServiceResponse{}
			(*pi).GetService(req, res)
			services = append(services, *res)
		}

		utils.JSONResponse(w, services)
	})

	mux.HandleFunc("GET /api/service/{tag}", func(w http.ResponseWriter, r *http.Request) {

		tag := r.PathValue("tag")

		pi, err := utils.GetPluginBySlug(tag, pis)
		if err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		request := serviceApi.RequestHandler(tag)

		response := &serviceApi.ServiceResponse{}

		(*pi).GetService(request, response)

		utils.JSONResponse(w, *response)
	})

	return mux
}
