package handlers

import (
	"fmt"
	"net/http"

	"github.com/nem-git/abcmovies/api"
	"github.com/nem-git/abcmovies/internal/drm/widevine"
	"github.com/nem-git/abcmovies/internal/utils"
)

func WidevineHandler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("POST /keys", func(w http.ResponseWriter, r *http.Request) {
		model := &api.WidevineKeysRequest{}

		if err := utils.BindJSON(r, model); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if model.Pssh == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: pssh"))
			return
		}
		if model.License == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: license"))
			return
		}
		if model.Headers == nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: headers"))
			return
		}

		keys, err := widevine.GetKeys(model.Pssh, model.License, model.Headers)
		if err != nil {
			api.InternalErrorHandler(w, err)
			return
		}

		response := api.WidevineKeysResponse{
			Keys: keys,
		}

		utils.JSONResponse(w, response)
	})
	mux.HandleFunc("POST /pssh", func(w http.ResponseWriter, r *http.Request) {
		model := &api.WidevinePsshRequest{}

		if err := utils.BindJSON(r, model); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if model.Url == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: url"))
			return
		}
		if model.Headers == nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: headers"))
			return
		}
		if model.SegHeaders == nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: segHeaders"))
			return
		}

		pssh, err := widevine.GetPssh(model.Url, model.Headers, model.SegHeaders)
		if err != nil {
			api.InternalErrorHandler(w, err)
			return
		}

		response := api.WidevinePsshResponse{
			Pssh: pssh,
		}

		utils.JSONResponse(w, response)
	})
	mux.HandleFunc("POST /remove", func(w http.ResponseWriter, r *http.Request) {
		model := &api.WidevineSegmentRequest{}

		if err := utils.BindJSON(r, model); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if model.InitStr == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: url"))
			return
		}
		if model.SegmentStr == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: headers"))
			return
		}
		if model.Keys == nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: segHeaders"))
			return
		}

		segment, err := widevine.GetDecryptedSegment(model.InitStr, model.SegmentStr, model.Keys)
		if err != nil {
			api.InternalErrorHandler(w, err)
			return
		}

		response := api.WidevineSegmentResponse{
			Segment: segment,
		}

		utils.JSONResponse(w, response)
	})

	return http.StripPrefix("/api/drm/widevine", mux)
}
