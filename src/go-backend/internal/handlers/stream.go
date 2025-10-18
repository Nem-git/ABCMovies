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

	contentType, err := (*p).GetStream(*req, model)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}
	if contentType == "" {
		contentType = config.MP4_CONTENT_TYPE
	}

	utils.ByteResponse(w, *model, contentType)
}
