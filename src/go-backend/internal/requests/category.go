package requests

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
)

type CategoryRequest struct {
	CategoryID string
}

func (r *CategoryRequest) Map(req *http.Request) error {

	r.CategoryID = req.PathValue(config.CATEGORY_SLUG)

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *CategoryRequest) Validate() error {
	if r.CategoryID == "" {
		return errs.ErrEmptyCategoryID
	}

	return nil
}

type CategoriesRequest struct {
}

func (r *CategoriesRequest) Map(req *http.Request) error {
	return nil
}

func (r *CategoriesRequest) Validate() error {
	return nil
}
