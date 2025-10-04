package requests

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/config"
)

type ShowRequest struct {
	ServiceTag string
	ShowID     string
}

func (r *ShowRequest) Map(req *http.Request) error {

	r.ServiceTag = req.PathValue(config.SERVICE_SLUG)
	r.ShowID = req.PathValue(config.SHOW_SLUG)

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *ShowRequest) Validate() error {
	if r.ServiceTag == "" {
		return ErrEmptyServiceTag
	}

	if r.ShowID == "" {
		return ErrEmptyShowID
	}

	return nil
}
