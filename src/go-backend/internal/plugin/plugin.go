package plugin

import (
	"github.com/nem-git/abcmovies/internal/episode"
	"github.com/nem-git/abcmovies/internal/recommendations/category"
	"github.com/nem-git/abcmovies/internal/search"
	"github.com/nem-git/abcmovies/internal/season"
	"github.com/nem-git/abcmovies/internal/service"
	"github.com/nem-git/abcmovies/internal/show"
	"github.com/nem-git/abcmovies/internal/stream"
)

type PluginInterface interface {
	GetService(req service.ServiceRequest, res *service.ServiceResponse)
	GetShow(req show.ShowRequest, res *show.ShowResponse)
	GetSeason(req season.SeasonRequest, res *season.SeasonResponse)
	GetEpisode(req episode.EpisodeRequest, res *episode.EpisodeResponse)

	GetNextEpisode(req episode.EpisodeRequest, res *episode.NextEpisodeResponse)

	GetStream(req stream.StreamRequest, res *stream.StreamResponse)

	GetSearch(req search.SearchRequest, res *search.SearchResponse)

	GetCategory(req category.CategoryRequest, res *category.CategoryResponse)
	GetCategories(req category.CategoriesRequest, res *category.CategoriesResponse)
}
