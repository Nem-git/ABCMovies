package providers

import (
	"fmt"
	"net/url"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/providers/cbc"
	"github.com/nem-git/abcmovies/internal/providers/stub"
	"github.com/nem-git/abcmovies/internal/providers/toutv"
)

func Build(cfg config.ServiceEntry, baseURL, apiPrefix string) (provider.Provider, error) {
	switch cfg.Type {
	case "stub":
		return buildStub(cfg, baseURL, apiPrefix), nil
	case "cbc":
		return buildCBC(cfg, baseURL, apiPrefix), nil
	case "toutv":
		return buildTouTV(cfg, baseURL, apiPrefix), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %q", cfg.Type)
	}
}

func buildStub(cfg config.ServiceEntry, baseURL, apiPrefix string) *stub.Provider {
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

	setStubImageURLs(baseURL, apiPrefix, cfg.Tag, sc.Movies, sc.Series, sc.Seasons)

	for _, se := range cfg.Search {
		item := oas.SearchResultItem{}
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

func setStubImageURLs(baseURL, apiPrefix, tag string, movies []oas.Movie, series []oas.Series, seasons []oas.Season) {
	for i := range movies {
		movies[i].Poster = stubImageURL(baseURL, apiPrefix, tag, "movies", movies[i].ID, "poster")
		movies[i].Backdrop = stubImageURL(baseURL, apiPrefix, tag, "movies", movies[i].ID, "backdrop")
	}
	for i := range series {
		series[i].Poster = stubImageURL(baseURL, apiPrefix, tag, "series", series[i].ID, "poster")
		series[i].Backdrop = stubImageURL(baseURL, apiPrefix, tag, "series", series[i].ID, "backdrop")
	}
	for i := range seasons {
		seasons[i].Poster = stubImageURL(baseURL, apiPrefix, tag, "series", seasons[i].ID, "poster")
		seasons[i].Backdrop = stubImageURL(baseURL, apiPrefix, tag, "series", seasons[i].ID, "backdrop")
	}
}

func stubImageURL(baseURL, apiPrefix, tag, resource, id, kind string) oas.OptURI {
	u, _ := url.Parse(baseURL + apiPrefix + fmt.Sprintf("/services/%s/%s/%s/%s", tag, resource, id, kind))
	return oas.NewOptURI(*u)
}

func buildCBC(cfg config.ServiceEntry, baseURL, apiPrefix string) *cbc.Provider {
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
		Tag:       cfg.Tag,
		Service:   &srv,
		BaseURL:   baseURL,
		APIPrefix: apiPrefix,
	})
}

func buildTouTV(cfg config.ServiceEntry, baseURL, apiPrefix string) *toutv.Provider {
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
	return toutv.New(toutv.Config{
		Tag:              cfg.Tag,
		Service:          &srv,
		SeriesCategoryID: "serie",
		MovieCategoryID:  "film",
		BaseURL:          baseURL,
		APIPrefix:        apiPrefix,
	})
}
