package requests

import (
	"net/http"
	"strconv"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
)

type StreamRequest struct {
	ServiceTag    string
	ShowID        string
	SeasonNumber  int
	EpisodeNumber int
	StreamID      string
}

func (r *StreamRequest) Map(req *http.Request) error {
	r.ServiceTag = req.PathValue(config.SERVICE_SLUG)
	r.ShowID = req.PathValue(config.SHOW_SLUG)

	var err error
	if r.SeasonNumber, err = strconv.Atoi(config.SEASON_SLUG); err != nil {
		return errs.ErrInvalidSeasonNumber
	}

	if r.EpisodeNumber, err = strconv.Atoi(config.EPISODE_SLUG); err != nil {
		return errs.ErrInvalidEpisodeNumber
	}

	r.StreamID = req.PathValue(config.STREAM_SLUG)

	if err = r.Validate(); err != nil {
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

	if r.SeasonNumber < 1 {
		return errs.ErrInvalidSeasonNumber
	}

	if r.EpisodeNumber < 1 {
		return errs.ErrInvalidEpisodeNumber
	}

	return nil
}
