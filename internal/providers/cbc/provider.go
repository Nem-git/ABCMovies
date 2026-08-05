package cbc

import (
	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/providers/rcmedia"
)

const appCode = "gem"

type Config struct {
	Tag     string
	Service *oas.Service

	SeriesCategoryID string
	MovieCategoryID  string

	BaseURL   string
	APIPrefix string
}

type Provider struct {
	*rcmedia.Provider
}

func New(cfg Config) *Provider {
	return &Provider{
		Provider: rcmedia.New(rcmedia.Config{
			Tag:              cfg.Tag,
			Service:          cfg.Service,
			AppCode:          appCode,
			SeriesCategoryID: cfg.SeriesCategoryID,
			MovieCategoryID:  cfg.MovieCategoryID,
			BaseURL:          cfg.BaseURL,
			APIPrefix:        cfg.APIPrefix,
		}),
	}
}
