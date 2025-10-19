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

	plugins, err := utils.GetPluginsContextValue[[]*plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	req, err := utils.GetRequestContextValue[requests.SearchRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	model := models.Search{
		Query: req.Query,
	}

	for _, p := range *plugins {

		m := models.Search{
			Query: req.Query,
		}

		if err := (*p).GetSearch(*req, &m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if m.Shows != nil {
			if model.Shows == nil {
				model.Shows = m.Shows
			} else {
				*model.Shows = append(*model.Shows, *m.Shows...)
			}
		}
	}

	if model.Shows != nil {
		model.ShowCount = len(*model.Shows)
	}

	utils.JSONResponse(w, &model)
}
