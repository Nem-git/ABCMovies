package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

type SearchHandler struct {
}

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	model := &models.Search{}

	p, err := utils.GetPluginContextValue[plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	req, err := utils.GetRequestContextValue[requests.SearchRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetSearch(*req, model); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, model)
}
