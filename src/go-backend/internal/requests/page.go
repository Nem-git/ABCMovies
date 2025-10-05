package requests

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
)

type PageRequest struct {
	PageID string
}

func (r *PageRequest) Map(req *http.Request) error {
	r.PageID = req.PathValue(config.PAGE_SLUG)

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *PageRequest) Validate() error {
	if r.PageID == "" {
		return errs.ErrEmptyPageID
	}

	return nil
}

type PagesRequest struct {
}

func (r *PagesRequest) Map(req *http.Request) error {
	return nil
}

func (r *PagesRequest) Validate() error {
	return nil
}
