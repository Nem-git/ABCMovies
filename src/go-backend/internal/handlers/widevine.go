package handlers

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/drm/widevine/keys"
	"github.com/nem-git/abcmovies/internal/drm/widevine/pssh"
	"github.com/nem-git/abcmovies/internal/drm/widevine/segment"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

type WidevineKeysHandler struct {
}

func (h *WidevineKeysHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	req, err := utils.GetRequestContextValue[requests.WidevineKeysRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	keys, err := keys.Get(req.PSSH, req.URL, req.Headers)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	model := &models.WidevineKeys{
		Keys: keys,
	}

	utils.JSONResponse(w, model)
}

type WidevinePSSHHandler struct {
}

func (h *WidevinePSSHHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	req, err := utils.GetRequestContextValue[requests.WidevinePSSHRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	pssh, err := pssh.Get(req.URL, req.Headers, req.SegHeaders)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	model := &models.WidevinePSSH{
		PSSH: pssh,
	}

	utils.JSONResponse(w, model)
}

type WidevineSegmentHandler struct {
}

func (h *WidevineSegmentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	req, err := utils.GetRequestContextValue[requests.WidevineSegmentRequest](r)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	segment, err := segment.Get(req.InitStr, req.SegmentStr, req.Keys, req.WantInit)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	model := &models.WidevineSegment{
		Segment: segment,
	}

	utils.JSONResponse(w, model)
}
