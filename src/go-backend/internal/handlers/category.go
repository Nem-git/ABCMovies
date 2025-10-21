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

type CategoryHandler struct {
	Plugins []plugin.IPlugin

	Request  models.CategoryRequest
	Response models.Category
}

func (h *CategoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetCategory(h.Request, &h.Response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, h.Response)
}

func (h *CategoryHandler) MapRequest(r *http.Request) error {

	h.Request.ServiceTag = r.PathValue(config.SERVICE_SLUG)
	h.Request.CategoryID = r.PathValue(config.CATEGORY_SLUG)

	if h.Request.ServiceTag == "" {
		return errs.ErrEmptyServiceTag
	}

	if h.Request.CategoryID == "" {
		return errs.ErrEmptyCategoryID
	}

	return nil
}

func (h *CategoryHandler) GetPlugin() (*plugin.IPlugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// /categories/{service}

type ServiceCategoriesHandler struct {
	Plugins []plugin.IPlugin

	Request  models.ServiceCategoriesRequest
	Response models.Categories
}

func (h *ServiceCategoriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetCategories(&h.Response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, h.Response)
}

func (h *ServiceCategoriesHandler) MapRequest(r *http.Request) error {

	h.Request.ServiceTag = r.PathValue(config.SERVICE_SLUG)

	if h.Request.ServiceTag == "" {
		return errs.ErrEmptyServiceTag
	}

	return nil
}

func (h *ServiceCategoriesHandler) GetPlugin() (*plugin.IPlugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// /categories

type CategoriesHandler struct {
	Plugins []plugin.IPlugin

	Response models.Categories
}

func (h *CategoriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	for _, p := range h.Plugins {

		m := &models.Categories{}

		if err := p.GetCategories(m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		h.Response.Categories = append(h.Response.Categories, m.Categories...)
	}

	h.Response.CategoryCount = len(h.Response.Categories)

	utils.JSONResponse(w, h.Response)
}
