package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

type CategoryHandler struct {
}

func (h *CategoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	model := &models.Category{}

	p, err := utils.GetPluginContextValue[plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	req, err := utils.GetRequestContextValue[requests.CategoryRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetCategory(*req, model); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, model)
}

type CategoriesHandler struct {
}

func (h *CategoriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	model := &models.Categories{}

	plugins, err := utils.GetPluginsContextValue[[]*plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	req, err := utils.GetRequestContextValue[requests.CategoryRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	for _, p := range *plugins {

		sr := requests.CategoryRequest{
			CategoryID: req.CategoryID,
		}
		m := &models.Category{}

		if err := (*p).GetCategory(sr, m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		model.Categories = append(model.Categories, m)
	}

	model.CategoryCount = len(model.Categories)

	utils.JSONResponse(w, model)
}
