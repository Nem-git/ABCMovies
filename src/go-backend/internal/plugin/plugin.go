package plugin

import (
	"github.com/nem-git/abcmovies/internal/models"
)

type IPlugin interface {
	GetServiceID() string

	GetService(*models.Service) error
	GetShow(models.ShowRequest, *models.Show) error
	GetSeason(models.SeasonRequest, *models.Season) error
	GetEpisode(models.EpisodeRequest, *models.Episode) error

	GetNextEpisode(models.EpisodeRequest, *models.NextEpisode) error

	GetStream(models.StreamRequest, *models.Stream) (string, error)

	GetSearch(models.SearchRequest, *models.Search) error

	GetCategory(models.CategoryRequest, *models.Category) error
	GetCategories(*models.Categories) error
}
