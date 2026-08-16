package convert

import (
	"github.com/nem-git/abcmovies/internal/convert/mp4"
	"github.com/nem-git/abcmovies/internal/drm"
	"github.com/nem-git/abcmovies/internal/proxy"
)

// NewRegistry builds the default converter registry for the given fetcher.
func NewRegistry(fetcher proxy.Fetcher) *Registry {
	return NewRegistryWithDRM(fetcher, nil)
}

// NewRegistryWithDRM builds a converter registry whose converters decrypt
// DRM-protected content with engine when non-nil.
func NewRegistryWithDRM(fetcher proxy.Fetcher, engine *drm.Engine) *Registry {
	r := newRegistry()
	m := mp4.NewConverterWithDRM(fetcher, engine)
	r.Register("dash", "mp4", m)
	r.Register("hls", "mp4", m)
	return r
}
