package requests

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
)

type CategoryRequest struct {
	ServiceTag string
	CategoryID string
}

func (r *CategoryRequest) Map(req *http.Request) error {

	r.ServiceTag = req.PathValue(config.SERVICE_SLUG)
	r.CategoryID = req.PathValue(config.CATEGORY_SLUG)

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *CategoryRequest) Validate() error {
	if r.ServiceTag == "" {
		return errs.ErrEmptyServiceTag
	}

	if r.CategoryID == "" {
		return errs.ErrEmptyCategoryID
	}

	return nil
}
