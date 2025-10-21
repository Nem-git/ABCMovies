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

type SearchHandler struct {
	Plugins []plugin.IPlugin

	Request  models.SearchRequest
	Response models.Search
}

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	for _, p := range h.Plugins {

		m := models.Search{
			Query: h.Request.Query,
		}

		if err := p.GetSearch(h.Request, &m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if m.Shows != nil {
			h.Response.Shows = append(h.Response.Shows, m.Shows...)
		}
	}

	if h.Response.Shows != nil {
		h.Response.ShowCount = len(h.Response.Shows)
	}

	utils.JSONResponse(w, h.Response)
}

func (h *SearchHandler) MapRequest(r *http.Request) error {

	h.Request.Query = r.PathValue(config.SEARCH_SLUG)

	if h.Request.Query == "" {
		return errs.ErrEmptySearchQuery
	}

	return nil
}
