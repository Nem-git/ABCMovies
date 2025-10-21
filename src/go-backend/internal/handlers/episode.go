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

type EpisodeHandler struct {
	Plugins []plugin.IPlugin

	Request  models.EpisodeRequest
	Response models.Episode
}

func (h *EpisodeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetEpisode(h.Request, &h.Response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, h.Response)
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

func (h *EpisodeHandler) GetPlugin() (*plugin.IPlugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

type NextEpisodeHandler struct {
	Plugins []plugin.IPlugin

	Request  models.EpisodeRequest
	Response models.NextEpisode
}

func (h *NextEpisodeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetNextEpisode(h.Request, &h.Response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, h.Response)
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

func (h *NextEpisodeHandler) GetPlugin() (*plugin.IPlugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}
