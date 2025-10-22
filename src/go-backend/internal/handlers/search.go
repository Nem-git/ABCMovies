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

func NewSearchHandler(plugins []plugin.Plugin) *SearchHandler {
	return &SearchHandler{Plugins: plugins}
}

type SearchHandler struct {
	Plugins []plugin.Plugin

	Request models.SearchRequest
}

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := models.Search{}

	for _, p := range h.Plugins {

		m := models.Search{
			Query: h.Request.Query,
		}

		if err := p.GetSearch(h.Request, &m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if m.Shows != nil {
			response.Shows = append(response.Shows, m.Shows...)
		}
	}

	if response.Shows != nil {
		response.ShowCount = len(response.Shows)
	}

	utils.JSONResponse(w, response)
}

func (h *SearchHandler) MapRequest(r *http.Request) error {

	h.Request.Query = r.PathValue(config.SEARCH_SLUG)

	if h.Request.Query == "" {
		return errs.ErrEmptySearchQuery
	}

	return nil
}
