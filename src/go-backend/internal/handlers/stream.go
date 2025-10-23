package handlers

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/utils"
)

func NewStreamHandler(plugins []plugin.Plugin) *StreamHandler {
	return &StreamHandler{Plugins: plugins}
}

type StreamHandler struct {
	Plugins []plugin.Plugin

	Request models.StreamRequest
}

func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	response := models.Stream{}

	p, err := h.GetPlugin()
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	contentType, err := (*p).GetStream(h.Request, &response)
	if err != nil {
		api.BadRequestErrorHandler(w, err)
		return
	}

	if contentType == "" {
		contentType = config.MP4_CONTENT_TYPE
	}

	utils.ByteResponse(w, response, contentType)
}

func (h *StreamHandler) MapRequest(r *http.Request) error {

	h.Request.ServiceTag = r.PathValue(config.SERVICE_SLUG)
	h.Request.ShowID = r.PathValue(config.SHOW_SLUG)

	var err error
	if h.Request.SeasonNumber, err = strconv.Atoi(r.PathValue(config.SEASON_SLUG)); err != nil {
		return errs.ErrInvalidSeasonNumber
	}

	if h.Request.EpisodeNumber, err = strconv.Atoi(r.PathValue(config.EPISODE_SLUG)); err != nil {
		return errs.ErrInvalidEpisodeNumber
	}

	h.Request.StreamType = r.PathValue(config.STREAM_SLUG)

	h.Request.StreamFileName = r.PathValue(config.STREAM_FILE_NAME_SLUG)

	h.Request.StreamMediaType = r.PathValue(config.STREAM_MEDIA_TYPE_SLUG)

	// Dash
	h.Request.StreamID = r.PathValue(config.STREAM_ID_SLUG)

	streamURL := r.PathValue(config.STREAM_URL_SLUG)

	parsedURL, err := utils.ParseStreamURL(streamURL)
	if h.Request.StreamFileName == "" {
		if err != nil {
			return err
		}

		uPath, err := url.ParseRequestURI(parsedURL)
		if err != nil {
			return errs.ErrInvalidURL
		}

		// Add request query params to stream URL
		queryParams := r.URL.Query()
		if len(queryParams) == 0 {
			h.Request.StreamURL = uPath.String()
		} else {
			h.Request.StreamURL = uPath.String() + "?" + queryParams.Encode()
		}
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

	if h.Request.StreamFileName != "" {
		if config.STREAM_TYPE_TO_FILE_NAME[h.Request.StreamType] != h.Request.StreamFileName {
			return errs.ErrInvalidStream
		}
	} else {
		if h.Request.StreamURL == "" {
			return errs.ErrInvalidURL
		}

		_, err := url.Parse(h.Request.StreamURL)
		if err != nil {
			return errs.ErrInvalidURL
		}
	}

	return nil
}

func (h *StreamHandler) GetPlugin() (*plugin.Plugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}
