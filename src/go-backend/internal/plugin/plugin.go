package plugin

import (
	episodeApi "github.com/nem-git/abcmovies/internal/episode/api"
	categoryApi "github.com/nem-git/abcmovies/internal/recommendations/category/api"
	searchApi "github.com/nem-git/abcmovies/internal/search/api"
	seasonApi "github.com/nem-git/abcmovies/internal/season/api"
	serviceApi "github.com/nem-git/abcmovies/internal/service/api"
	showApi "github.com/nem-git/abcmovies/internal/show/api"
	streamApi "github.com/nem-git/abcmovies/internal/stream/api"
)

type PluginInterface interface {
	GetServiceSlug() string

	GetService(req serviceApi.ServiceRequest, res *serviceApi.ServiceResponse) error
	GetShow(req showApi.ShowRequest, res *showApi.ShowResponse) error
	GetSeason(req seasonApi.SeasonRequest, res *seasonApi.SeasonResponse) error
	GetEpisode(req episodeApi.EpisodeRequest, res *episodeApi.EpisodeResponse) error

	GetNextEpisode(req episodeApi.EpisodeRequest, res *episodeApi.NextEpisodeResponse) error

	GetStream(req streamApi.StreamRequest, res *streamApi.StreamResponse) error

	GetSearch(req searchApi.SearchRequest, res *searchApi.SearchResponse) error

	GetCategory(req categoryApi.CategoryRequest, res *categoryApi.CategoryResponse) error
	GetCategories(req categoryApi.CategoriesRequest, res *categoryApi.CategoriesResponse) error
}
