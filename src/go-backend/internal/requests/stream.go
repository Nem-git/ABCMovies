package requests

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/utils"
)

type StreamRequest struct {
	ServiceTag     string
	ShowID         string
	SeasonNumber   int
	EpisodeNumber  int
	StreamType     string
	StreamFileName string
	StreamURL      string
}

func (r *StreamRequest) Map(req *http.Request) error {
	r.ServiceTag = req.PathValue(config.SERVICE_SLUG)
	r.ShowID = req.PathValue(config.SHOW_SLUG)

	var err error
	if r.SeasonNumber, err = strconv.Atoi(req.PathValue(config.SEASON_SLUG)); err != nil {
		return errs.ErrInvalidSeasonNumber
	}

	if r.EpisodeNumber, err = strconv.Atoi(req.PathValue(config.EPISODE_SLUG)); err != nil {
		return errs.ErrInvalidEpisodeNumber
	}

	r.StreamType = req.PathValue(config.STREAM_SLUG)

	r.StreamFileName = req.PathValue(config.STREAM_FILE_NAME_SLUG)

	streamURL := req.PathValue(config.STREAM_URL_SLUG)

	parsedURL, err := utils.ParseStreamURL(streamURL)
	if r.StreamFileName == "" {
		if err != nil {
			return err
		}

		uPath, err := url.ParseRequestURI(parsedURL)
		if err != nil {
			return errs.ErrInvalidURL
		}

		// Add request query params to stream URL
		queryParams := req.URL.Query()
		if len(queryParams) == 0 {
			r.StreamURL = uPath.String()
		} else {
			r.StreamURL = uPath.String() + "?" + queryParams.Encode()
		}
	}

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *StreamRequest) Validate() error {
	if r.ServiceTag == "" {
		return errs.ErrEmptyServiceTag
	}

	if r.ShowID == "" {
		return errs.ErrEmptyShowID
	}

	if r.SeasonNumber < 0 {
		return errs.ErrInvalidSeasonNumber
	}

	if r.EpisodeNumber < 0 {
		return errs.ErrInvalidEpisodeNumber
	}

	if r.StreamFileName != "" {
		if config.STREAM_TYPE_TO_FILE_NAME[r.StreamType] != r.StreamFileName {
			return errs.ErrInvalidStream
		}
	} else {
		if r.StreamURL == "" {
			return errs.ErrInvalidURL
		}

		_, err := url.Parse(r.StreamURL)
		if err != nil {
			return errs.ErrInvalidURL
		}
	}

	return nil
}
