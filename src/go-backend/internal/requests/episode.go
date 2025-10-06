package requests

import (
	"net/http"
	"strconv"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
)

type EpisodeRequest struct {
	ServiceTag    string
	ShowID        string
	SeasonNumber  int
	EpisodeNumber int
}

func (r *EpisodeRequest) Map(req *http.Request) error {
	r.ServiceTag = req.PathValue(config.SERVICE_SLUG)
	r.ShowID = req.PathValue(config.SHOW_SLUG)

	var err error
	if r.SeasonNumber, err = strconv.Atoi(req.PathValue(config.SEASON_SLUG)); err != nil {
		return errs.ErrInvalidSeasonNumber
	}

	if r.EpisodeNumber, err = strconv.Atoi(req.PathValue(config.EPISODE_SLUG)); err != nil {
		return errs.ErrInvalidEpisodeNumber
	}

	if err = r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *EpisodeRequest) Validate() error {
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

	return nil
}
