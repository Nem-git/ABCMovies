package handlers

import (
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

	model := &models.Service{}

	p, err := utils.GetPluginContextValue[plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	req, err := utils.GetRequestContextValue[requests.ServiceRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetService(*req, model); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, model)
}

type ServicesHandler struct {
}

func (h *ServicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	model := &models.Services{}

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

	for _, p := range *plugins {

		sr := requests.ServiceRequest{
			ServiceTag: (*p).GetServiceID(),
		}
		m := &models.Service{}

		if err := (*p).GetService(sr, m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if model.Services == nil {
			model.Services = []models.Service{*m}
		} else {
			model.Services = append(model.Services, *m)
		}
	}

	if model.Services != nil {
		model.ServiceCount = len(model.Services)
	}

	utils.JSONResponse(w, model)
}
