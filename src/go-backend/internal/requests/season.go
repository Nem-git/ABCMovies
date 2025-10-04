package requests

import (
	"net/http"
	"strconv"

	"github.com/nem-git/abcmovies/internal/config"
)

type SeasonRequest struct {
	ServiceTag   string
	ShowID       string
	SeasonNumber int
}

func (r *SeasonRequest) Map(req *http.Request) error {

	r.ServiceTag = req.PathValue(config.SERVICE_SLUG)
	r.ShowID = req.PathValue(config.SHOW_SLUG)

	var err error
	if r.SeasonNumber, err = strconv.Atoi(config.SEASON_SLUG); err != nil {
		return ErrInvalidSeasonNumber
	}

	if err = r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *SeasonRequest) Validate() error {
	if r.ServiceTag == "" {
		return ErrEmptyServiceTag
	}

	if r.ShowID == "" {
		return ErrEmptyShowID
	}

	if r.SeasonNumber < 1 {
		return ErrInvalidSeasonNumber
	}

	return nil
}
