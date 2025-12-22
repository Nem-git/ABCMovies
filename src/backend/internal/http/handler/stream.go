package handler

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/http/api"
	"github.com/nem-git/abcmovies/internal/http/model"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/storage/cache/connector"
	dashController "github.com/nem-git/abcmovies/internal/storage/cache/controller/dash"
	dashRepo "github.com/nem-git/abcmovies/internal/storage/cache/repository/dash"
	"github.com/nem-git/abcmovies/internal/utils"
)

func NewStreamHandler(plugins []plugin.Plugin) *StreamHandler {
	return &StreamHandler{Plugins: plugins}
}

type StreamHandler struct {
	Plugins []plugin.Plugin

	Request model.StreamRequest
}

func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	switch h.Request.StreamType {
	case config.STREAM_DASH_TYPE:
		if h.Request.IsPlaylist {
			conn := connector.NewRedisConnector(connector.ConnectionDetails{
				Address:  config.TEMP_REDIS_ADDRESS,
				User:     config.TEMP_REDIS_USER,
				Password: config.TEMP_REDIS_PASSWORD,
				DB:       config.TEMP_REDIS_DB,
			})
			repo := dashRepo.NewManifestRepository(conn)
			controller := dashController.NewManifestController(repo)

			dbID := utils.GetUniqueStreamPlaylistID(h.Request)

			m, err := controller.ReadSingle(dbID)
			if err == nil {
				utils.ByteResponse(w, []byte(m), config.DASH_CONTENT_TYPE)
				return
			}
		}

		// if h.Request.Media.Dash.Type == config.DASH_INIT_URL_PREFIX {
		// 	init, err := segment.GetUsingDB(h.Request.Media.Dash.ID)
		// 	if err == nil {
		// 		utils.ByteResponse(w, init, config.DASH_CONTENT_TYPE)
		// 		return
		// 	}
		// }
	}

	response := model.Stream{}

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

	h.Request.Playlist.FileName = r.PathValue(config.STREAM_FILE_NAME_SLUG)
	if h.Request.Playlist.FileName != "" {
		h.Request.IsPlaylist = true
	}

	streamURL := r.PathValue(config.STREAM_URL_SLUG)
	if streamURL != "" {
		h.Request.IsMedia = true

		parsedURL, err := utils.ParseStreamURL(streamURL)
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
			h.Request.Media.URL = uPath.String()
		} else {
			h.Request.Media.URL = uPath.String() + "?" + queryParams.Encode()
		}
	}

	// Dash
	h.Request.Media.Dash.Type = r.PathValue(config.STREAM_MEDIA_TYPE_SLUG)

	h.Request.Media.Dash.ID = r.PathValue(config.STREAM_ID_SLUG)

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

	if h.Request.Playlist.FileName != "" {
		// Verify if ex: manifest.mpd is indeed dash's playlist file name
		if config.STREAM_TYPE_TO_FILE_NAME[h.Request.StreamType] != h.Request.Playlist.FileName {
			return errs.ErrInvalidStream
		}
	} else {
		if h.Request.Media.URL == "" {
			return errs.ErrInvalidURL
		}

		_, err := url.Parse(h.Request.Media.URL)
		if err != nil {
			return errs.ErrInvalidURL
		}
	}

	// Dash
	// TODO: Add DASH

	return nil
}

func (h *StreamHandler) GetPlugin() (*plugin.Plugin, error) {
	p, err := plugin.GetByID(h.Request.ServiceTag, h.Plugins)
	if err != nil {
		return nil, err
	}

	return &p, nil
}
