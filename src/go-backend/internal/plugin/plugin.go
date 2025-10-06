package plugin

import (
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/requests"
)

type IPlugin interface {
	GetServiceID() string

	GetService(requests.ServiceRequest, *models.Service) error
	GetShow(requests.ShowRequest, *models.Show) error
	GetSeason(requests.SeasonRequest, *models.Season) error
	GetEpisode(requests.EpisodeRequest, *models.Episode) error

	GetNextEpisode(requests.EpisodeRequest, *models.NextEpisode) error

	GetStream(requests.StreamRequest, *models.Stream) error

	GetSearch(requests.SearchRequest, *models.Search) error

	GetCategory(requests.CategoryRequest, *models.Category) error
	GetCategories(requests.CategoriesRequest, *models.Categories) error
}
