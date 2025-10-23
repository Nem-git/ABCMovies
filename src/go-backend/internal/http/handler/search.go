package handler

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/http/api"
	"github.com/nem-git/abcmovies/internal/http/model"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/utils"
)

func NewSearchHandler(plugins []plugin.Plugin) *SearchHandler {
	return &SearchHandler{Plugins: plugins}
}

type SearchHandler struct {
	Plugins []plugin.Plugin

	Request model.SearchRequest
}

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := model.Search{}

	for _, p := range h.Plugins {

		m := model.Search{
			Query: h.Request.Query,
		}

		if err := p.GetSearch(h.Request, &m); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		tag := p.GetServiceID()

		results := []model.SearchResult{}

		for _, s := range m.Shows {
			s.ServiceTag = tag
			results = append(results, s)
		}

		if m.Shows != nil {
			response.Shows = append(response.Shows, results...)
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

func NewServiceSearchHandler(plugins []plugin.Plugin) *ServiceSearchHandler {
	return &ServiceSearchHandler{Plugins: plugins}
}

type ServiceSearchHandler struct {
	Plugins []plugin.Plugin

	Request model.ServiceSearchRequest
}

func (h *ServiceSearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := model.Search{}

	for _, p := range h.Plugins {

		m := model.Search{
			Query: h.Request.Query,
		}

		if err := p.GetSearch(h.Request.SearchRequest, &m); err != nil {
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

func (h *ServiceSearchHandler) MapRequest(r *http.Request) error {

	h.Request.ServiceTag = r.PathValue(config.SERVICE_SLUG)

	if h.Request.ServiceTag == "" {
		return errs.ErrEmptyServiceTag
	}

	h.Request.Query = r.PathValue(config.SEARCH_SLUG)

	if h.Request.Query == "" {
		return errs.ErrEmptySearchQuery
	}

	return nil
}
