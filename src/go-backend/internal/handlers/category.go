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

func NewCategoryHandler(plugins []plugin.IPlugin) *CategoryHandler {
	return &CategoryHandler{Plugins: plugins}
}

type CategoryHandler struct {
	Plugins []plugin.IPlugin

	Request models.CategoryRequest
}

func (h *CategoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := models.Category{}

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetCategory(h.Request, &response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, response)
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

func NewServiceCategoryHandler(plugins []plugin.IPlugin) *ServiceCategoriesHandler {
	return &ServiceCategoriesHandler{Plugins: plugins}
}

type ServiceCategoriesHandler struct {
	Plugins []plugin.IPlugin

	Request models.ServiceCategoriesRequest
}

func (h *ServiceCategoriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := models.Categories{}

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetCategories(&response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, response)
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

func NewCategoriesHandler(plugins []plugin.IPlugin) *CategoriesHandler {
	return &CategoriesHandler{Plugins: plugins}
}

type CategoriesHandler struct {
	Plugins []plugin.IPlugin
}

func (h *CategoriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := models.Categories{}

	for _, p := range h.Plugins {

		m := &models.Categories{}

		if err := p.GetCategories(m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		// Adds tag

		tag := p.GetServiceID()

		categories := []models.Category{}

		for _, c := range m.Categories {
			c.ServiceTag = tag
			categories = append(categories, c)
		}

		response.Categories = append(response.Categories, categories...)
	}

	response.CategoryCount = len(response.Categories)

	utils.JSONResponse(w, &response)
}
