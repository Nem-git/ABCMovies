package providers

import (
	"fmt"
	"net/url"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/providers/cbc"
	"github.com/nem-git/abcmovies/internal/providers/stub"
)

func Build(cfg config.ServiceEntry) (provider.Provider, error) {
	switch cfg.Type {
	case "stub":
		return buildStub(cfg), nil
	case "cbc":
		return buildCBC(cfg), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %q", cfg.Type)
	}
}

func buildStub(cfg config.ServiceEntry) *stub.Provider {
	srv := oas.Service{
		Tag:  cfg.Tag,
		Name: cfg.Name,
	}
	if cfg.Description != "" {
		srv.Description = oas.NewOptString(cfg.Description)
	}
	if cfg.Country != "" {
		srv.Country = oas.NewOptString(cfg.Country)
	}
	if len(cfg.Languages) > 0 {
		srv.Languages = cfg.Languages
	}
	if cfg.URL != "" {
		if u, err := url.Parse(cfg.URL); err == nil {
			srv.Website = oas.NewOptURI(*u)
		}
	}

	sc := stub.Config{
		Tag:       cfg.Tag,
		Service:   &srv,
		Movies:    cfg.Movies,
		Series:    cfg.Series,
		Seasons:   cfg.Seasons,
		Episodes:  cfg.Episodes,
		Streams:   cfg.Streams,
		Subtitles: cfg.Subtitles,
	}

	for _, se := range cfg.Search {
		item := oas.SearchResultItem{Score: float32(se.Score)}
		switch se.ResourceType {
		case "Movie":
			for _, m := range cfg.Movies {
				if m.ID == se.ResourceID {
					item.Resource.SetMovie(m)
					break
				}
			}
		case "TVSeries":
			for _, s := range cfg.Series {
				if s.ID == se.ResourceID {
					item.Resource.SetSeries(s)
					break
				}
			}
		}
		if item.Resource.Type != "" {
			sc.Search = append(sc.Search, item)
		}
	}

	return stub.New(sc)
}

func buildCBC(cfg config.ServiceEntry) *cbc.Provider {
	srv := oas.Service{
		Tag:  cfg.Tag,
		Name: cfg.Name,
	}
	if cfg.Description != "" {
		srv.Description = oas.NewOptString(cfg.Description)
	}
	if cfg.Country != "" {
		srv.Country = oas.NewOptString(cfg.Country)
	}
	if len(cfg.Languages) > 0 {
		srv.Languages = cfg.Languages
	}
	if cfg.URL != "" {
		if u, err := url.Parse(cfg.URL); err == nil {
			srv.Website = oas.NewOptURI(*u)
		}
	}
	return cbc.New(cbc.Config{
		Tag:     cfg.Tag,
		Service: &srv,
	})
}
