package registry

import (
	"errors"

	"github.com/nem-git/abcmovies/internal/provider"
)

var ErrDuplicateTag = errors.New("a service using that tag already exists")
var ErrNotFound = errors.New("no service corresponding to that tag was registered")

func New() *Registry {
	return &Registry{}
}

type Registry struct {
	providers []provider.Provider
}

func (r *Registry) Register(p provider.Provider) error {
	for _, existing := range r.providers {
		if existing.Tag() == p.Tag() {
			return ErrDuplicateTag
		}
	}

	r.providers = append(r.providers, p)

	return nil
}

func (r *Registry) Get(tag string) (provider.Provider, error) {
	for _, p := range r.providers {
		if p.Tag() == tag {
			return p, nil
		}
	}

	return nil, ErrNotFound
}

func (r *Registry) All() []provider.Provider {
	result := make([]provider.Provider, len(r.providers))
	copy(result, r.providers)

	return result
}
