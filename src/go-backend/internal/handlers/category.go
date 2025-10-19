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

	p, err := utils.GetPluginContextValue[plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	model := &models.Categories{}

	if err := (*p).GetCategories(model); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if model.Categories != nil {
		model.CategoryCount = len(model.Categories)
	}

	utils.JSONResponse(w, model)
}

// func (h *CategoriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

// 	log.Println(r.Context())

// 	plugins, err := utils.GetPluginsContextValue[[]*plugin.IPlugin](r)
// 	if err != nil {
// 		api.BadRequestErrorHandler(w, err)
// 		return
// 	}

// 	model := &models.Categories{}

// 	for _, p := range *plugins {

// 		m := &models.Categories{}

// 		if err := (*p).GetCategories(m); err != nil {
// 			api.BadRequestErrorHandler(w, err)
// 			return
// 		}

// 		model.Categories = append(model.Categories, m.Categories...)
// 	}

// 	if model.Categories != nil {
// 		model.CategoryCount = len(model.Categories)
// 	}

// 	utils.JSONResponse(w, model)
// }
