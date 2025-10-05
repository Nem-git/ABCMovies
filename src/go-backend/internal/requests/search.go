package requests

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
)

type SearchRequest struct {
	Query string
}

func (r *SearchRequest) Map(req *http.Request) error {
	r.Query = req.PathValue(config.SEARCH_SLUG)

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *SearchRequest) Validate() error {
	if r.Query == "" {
		return errs.ErrEmptySearchQuery
	}

	return nil
}
