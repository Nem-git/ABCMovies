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

	GetService(req serviceApi.ServiceRequest, res *serviceApi.ServiceResponse)
	GetShow(req showApi.ShowRequest, res *showApi.ShowResponse)
	GetSeason(req seasonApi.SeasonRequest, res *seasonApi.SeasonResponse)
	GetEpisode(req episodeApi.EpisodeRequest, res *episodeApi.EpisodeResponse)

	GetNextEpisode(req episodeApi.EpisodeRequest, res *episodeApi.NextEpisodeResponse)

	GetStream(req streamApi.StreamRequest, res *streamApi.StreamResponse)

	GetSearch(req searchApi.SearchRequest, res *searchApi.SearchResponse)

	GetCategory(req categoryApi.CategoryRequest, res *categoryApi.CategoryResponse)
	GetCategories(req categoryApi.CategoriesRequest, res *categoryApi.CategoriesResponse)
}
