package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

type ShowHandler struct {
}

func (h *ShowHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	model := &models.Show{}

	p, err := utils.GetPluginContextValue[plugin.IPlugin](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	req, err := utils.GetRequestContextValue[requests.ShowRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if err := (*p).GetShow(*req, model); err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	utils.JSONResponse(w, model)
}
