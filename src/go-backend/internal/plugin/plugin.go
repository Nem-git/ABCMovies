package plugin

import (
	"github.com/nem-git/abcmovies/internal/models"
	"github.com/nem-git/abcmovies/internal/requests"
)

type IPlugin interface {
	GetServiceID() string

	GetService(requests.ServiceRequest, *models.Service) error
	GetShow(req requests.ShowRequest, res *models.Show) error
	GetSeason(req requests.SeasonRequest, res *models.Season) error
	GetEpisode(req requests.EpisodeRequest, res *models.Episode) error

	GetNextEpisode(req requests.EpisodeRequest, res *models.NextEpisode) error

	GetStream(req requests.StreamRequest, res *models.Stream) error

	GetSearch(req requests.SearchRequest, res *models.Search) error

	GetCategory(req requests.CategoryRequest, res *models.Category) error
	GetCategories(req requests.CategoriesRequest, res *models.Categories) error
}
