package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

type EpisodeHandler struct {
}

func (h *EpisodeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	model := &models.Episode{}

	p, err := utils.GetPluginContextValue[plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	req, err := utils.GetRequestContextValue[requests.EpisodeRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetEpisode(*req, model); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, model)
}
