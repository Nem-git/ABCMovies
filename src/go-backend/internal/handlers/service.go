package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/utils"
)

func NewServiceHandler(plugins []plugin.IPlugin) *ServiceHandler {
	return &ServiceHandler{Plugins: plugins}
}

type ServiceHandler struct {
	Plugins []plugin.IPlugin

	Request models.ServiceRequest
}

func (h *ServiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := models.Service{}

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetService(&response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, response)
}

func (h *ServiceHandler) MapRequest(r *http.Request) error {

	h.Request.ServiceTag = r.PathValue(config.SERVICE_SLUG)

	if h.Request.ServiceTag == "" {
		return errs.ErrEmptyServiceTag
	}

	// TODO: Add verification of validity of tag

	return nil
}

func (h *ServiceHandler) GetPlugin() (*plugin.IPlugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func NewServicesHandler(plugins []plugin.IPlugin) *ServicesHandler {
	return &ServicesHandler{Plugins: plugins}
}

type ServicesHandler struct {
	Plugins []plugin.IPlugin
}

func (h *ServicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := models.Services{}

	for _, p := range h.Plugins {

		m := &models.Service{}

		if err := p.GetService(m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		response.Services = append(response.Services, *m)
	}

	if response.Services != nil {
		response.ServiceCount = len(response.Services)
	}

	utils.JSONResponse(w, response)
}
