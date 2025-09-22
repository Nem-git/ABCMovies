package widevine

import (
	"fmt"
	"log"
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/drm/widevine/keys"
	"github.com/nem-git/abcmovies/internal/drm/widevine/pssh"
	"github.com/nem-git/abcmovies/internal/drm/widevine/segment"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("POST /keys", func(w http.ResponseWriter, r *http.Request) {
		model := &KeysRequest{}

		if err := utils.BindJSON(r, model); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if model.Pssh == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: pssh"))
			return
		}
		if model.URL == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: url"))
			return
		}
		if model.Headers == nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: headers"))
			return
		}

		log.Println(*model)

		keys, err := keys.Get(model.Pssh, model.URL, model.Headers)
		if err != nil {
			api.InternalErrorHandler(w, err)
			return
		}

		response := KeysResponse{
			Keys: keys,
		}

		utils.JSONResponse(w, response)
	})
	mux.HandleFunc("POST /pssh", func(w http.ResponseWriter, r *http.Request) {
		model := &PsshRequest{}

		if err := utils.BindJSON(r, model); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if model.URL == "" {
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

		pssh, err := pssh.Get(model.URL, model.Headers, model.SegHeaders)
		if err != nil {
			api.InternalErrorHandler(w, err)
			return
		}

		response := PsshResponse{
			Pssh: pssh,
		}

		utils.JSONResponse(w, response)
	})
	mux.HandleFunc("POST /remove", func(w http.ResponseWriter, r *http.Request) {
		model := &SegmentRequest{}

		if err := utils.BindJSON(r, model); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		if model.SegmentStr == "" {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: segment"))
			return
		}
		if model.Keys == nil {
			api.BadRequestErrorHandler(w, fmt.Errorf("missing parameter: keys"))
			return
		}

		segment, err := segment.Get(model.InitStr, model.SegmentStr, model.Keys, model.IsInit)
		if err != nil {
			api.InternalErrorHandler(w, err)
			return
		}

		//log.Println(segment)

		// err = os.WriteFile("f.mp4", []byte(segment), 0644)
		// if err != nil {
		// 	panic(err)
		// }

		response := SegmentResponse{
			Segment: segment,
		}

		utils.JSONResponse(w, response)
	})

	return http.StripPrefix("/api/drm/widevine", mux)
}
