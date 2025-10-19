package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

// TODO: Add pages

type PageHandler struct {
}

func (h *PageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	model := &models.Page{}

	_, err := utils.GetPluginContextValue[plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	_, err = utils.GetRequestContextValue[requests.PageRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	// if err := (*p).GetCategories(*req, model); err != nil {
	// 	api.BadRequestErrorHandler(w, err)
	// 	return
	// }

	utils.JSONResponse(w, model)
}

type PagesHandler struct {
}

func (h *PagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	model := &models.Pages{}

	_, err := utils.GetPluginsContextValue[[]*plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	// for _, p := range *plugins {

	// 	sr := requests.PageRequest{
	// 		PageID: req.PageID,
	// 	}
	// 	m := &models.Page{}

	// 	if err := (*p).GetPage(sr, m); err != nil {
	// 		api.BadRequestErrorHandler(w, err)
	// 		return
	// 	}

	// 	model.Pages = append(model.Pages, m)
	// }

	model.PageCount = len(model.Pages)

	utils.JSONResponse(w, model)
}
