package handlers

import (
	"net/http"
	"strconv"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/utils"
)

func NewEpisodeHandler(plugins []plugin.Plugin) *EpisodeHandler {
	return &EpisodeHandler{Plugins: plugins}
}

type EpisodeHandler struct {
	Plugins []plugin.Plugin

	Request models.EpisodeRequest
}

func (h *EpisodeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := models.Episode{}

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetEpisode(h.Request, &response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, response)
}

func (h *EpisodeHandler) MapRequest(r *http.Request) error {

	h.Request.ServiceTag = r.PathValue(config.SERVICE_SLUG)
	h.Request.ShowID = r.PathValue(config.SHOW_SLUG)

	var err error
	if h.Request.SeasonNumber, err = strconv.Atoi(r.PathValue(config.SEASON_SLUG)); err != nil {
		return errs.ErrInvalidSeasonNumber
	}

	if h.Request.EpisodeNumber, err = strconv.Atoi(r.PathValue(config.EPISODE_SLUG)); err != nil {
		return errs.ErrInvalidEpisodeNumber
	}

	if h.Request.ServiceTag == "" {
		return errs.ErrEmptyServiceTag
	}

	if h.Request.ShowID == "" {
		return errs.ErrEmptyShowID
	}

	if h.Request.SeasonNumber < 0 {
		return errs.ErrInvalidSeasonNumber
	}

	if h.Request.EpisodeNumber < 0 {
		return errs.ErrInvalidEpisodeNumber
	}

	return nil
}

func (h *EpisodeHandler) GetPlugin() (*plugin.Plugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func NewNextEpisodeHandler(plugins []plugin.Plugin) *NextEpisodeHandler {
	return &NextEpisodeHandler{Plugins: plugins}
}

type NextEpisodeHandler struct {
	Plugins []plugin.Plugin

	Request models.EpisodeRequest
}

func (h *NextEpisodeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := models.NextEpisode{}

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetNextEpisode(h.Request, &response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, response)
}

func (h *NextEpisodeHandler) MapRequest(r *http.Request) error {

	h.Request.ServiceTag = r.PathValue(config.SERVICE_SLUG)
	h.Request.ShowID = r.PathValue(config.SHOW_SLUG)

	var err error
	if h.Request.SeasonNumber, err = strconv.Atoi(r.PathValue(config.SEASON_SLUG)); err != nil {
		return errs.ErrInvalidSeasonNumber
	}

	if h.Request.EpisodeNumber, err = strconv.Atoi(r.PathValue(config.EPISODE_SLUG)); err != nil {
		return errs.ErrInvalidEpisodeNumber
	}

	if h.Request.ServiceTag == "" {
		return errs.ErrEmptyServiceTag
	}

	if h.Request.ShowID == "" {
		return errs.ErrEmptyShowID
	}

	if h.Request.SeasonNumber < 0 {
		return errs.ErrInvalidSeasonNumber
	}

	if h.Request.EpisodeNumber < 0 {
		return errs.ErrInvalidEpisodeNumber
	}

	return nil
}

func (h *NextEpisodeHandler) GetPlugin() (*plugin.Plugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}
