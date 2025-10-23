package plugin

import "github.com/nem-git/abcmovies/internal/http/model"

type Plugin interface {
	GetServiceID() string

	GetService(*model.Service) error
	GetShow(model.ShowRequest, *model.Show) error
	GetSeason(model.SeasonRequest, *model.Season) error
	GetEpisode(model.EpisodeRequest, *model.Episode) error

	GetNextEpisode(model.EpisodeRequest, *model.NextEpisode) error

	GetStream(model.StreamRequest, *model.Stream) (string, error)

	GetSearch(model.SearchRequest, *model.Search) error

	GetCategory(model.CategoryRequest, *model.Category) error
	GetCategories(*model.Categories) error
}
