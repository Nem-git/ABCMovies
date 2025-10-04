package requests

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/utils"
)

type DashManifestRequest struct {
	URL     string `json:"url"`
	Content string `json:"content"`
}

func (r *DashManifestRequest) Map(req *http.Request) error {
	if err := utils.BindJSON(req, r); err != nil {
		return err
	}

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *DashManifestRequest) Validate() error {
	if r.URL == "" {
		return ErrEmptyDashManifestURL
	}

	if r.Content == "" {
		return ErrEmptyDashManifestContent
	}

	return nil
}
