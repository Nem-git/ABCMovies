package handlers

import (
	"log"
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

type ServiceHandler struct {
}

func (h *ServiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	p, err := utils.GetPluginContextValue[plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	req, err := utils.GetRequestContextValue[requests.ServiceRequest](r)
	if err != nil {
		log.Printf("THING THING 2")
		api.BadRequestErrorHandler(w, err)
		return
	}

	service := &models.Service{}

	if err := (*p).GetService(*req, service); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, *service)
}

type ServicesHandler struct {
}

func (h *ServicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	plugins, err := utils.GetPluginsContextValue[[]*plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	_, err = utils.GetRequestContextValue[requests.ServicesRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	services := &models.Services{}

	for _, p := range *plugins {
		service := &models.Service{}
		if err := (*p).GetService(requests.ServiceRequest{}, service); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		services.Services = append(services.Services, *service)
	}

	utils.JSONResponse(w, services)
}
