package requests

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/config"
)

type ServiceRequest struct {
	ServiceTag string
}

func (r *ServiceRequest) Map(req *http.Request) error {

	r.ServiceTag = req.PathValue(config.SERVICE_SLUG)

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *ServiceRequest) Validate() error {
	if r.ServiceTag == "" {
		return ErrEmptyServiceTag
	}

	// TODO: Add verification of validity of tag

	return nil
}

type ServicesRequest struct {
}

func (r *ServicesRequest) Map(req *http.Request) error {
	return nil
}

func (r *ServicesRequest) Validate() error {
	return nil
}
