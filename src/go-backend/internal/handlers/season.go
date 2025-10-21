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

type SeasonHandler struct {
	Plugins []plugin.IPlugin

	Request  models.SeasonRequest
	Response models.Season
}

func (h *SeasonHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetSeason(h.Request, &h.Response); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, h.Response)
}

func (h *SeasonHandler) MapRequest(r *http.Request) error {

	h.Request.ServiceTag = r.PathValue(config.SERVICE_SLUG)
	h.Request.ShowID = r.PathValue(config.SHOW_SLUG)

	var err error
	if h.Request.SeasonNumber, err = strconv.Atoi(r.PathValue(config.SEASON_SLUG)); err != nil {
		return errs.ErrInvalidSeasonNumber
	}

	if h.Request.ServiceTag == "" {
		return errs.ErrEmptyServiceTag
	}

	if h.Request.ShowID == "" {
		return errs.ErrEmptyShowID
	}

	// fuck people who do that!
	if h.Request.SeasonNumber < 0 {
		return errs.ErrInvalidSeasonNumber
	}

	return nil
}

func (h *SeasonHandler) GetPlugin() (*plugin.IPlugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}
