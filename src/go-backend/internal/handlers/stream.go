package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

type StreamHandler struct {
}

func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	model := &models.Stream{}

	p, err := utils.GetPluginContextValue[plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	req, err := utils.GetRequestContextValue[requests.StreamRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetStream(*req, model); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	// TODO: Figure out the right content-type

	utils.ByteResponse(w, *model, config.DASH_CONTENT_TYPE)
}
