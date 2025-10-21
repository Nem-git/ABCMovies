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

type ServiceHandler struct {
	Plugins []plugin.IPlugin

	Request  models.ServiceRequest
	Response models.Service
}

func (h *ServiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetService(&h.Response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, h.Response)
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

type ServicesHandler struct {
	Plugins []plugin.IPlugin

	Response models.Services
}

func (h *ServicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	for _, p := range h.Plugins {

		m := &models.Service{}

		if err := p.GetService(m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		h.Response.Services = append(h.Response.Services, *m)
	}

	if h.Response.Services != nil {
		h.Response.ServiceCount = len(h.Response.Services)
	}

	utils.JSONResponse(w, h.Response)
}
