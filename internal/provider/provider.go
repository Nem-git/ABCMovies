package provider

import (
	"context"
	"errors"
	"io"

	"github.com/nem-git/abcmovies/internal/oas"
)

var ErrNotSupported = errors.New("resource type not supported by this provider or not yet implemented in provider")

type Provider interface {
	Tag() string
	Service(ctx context.Context) (*oas.Service, error)
	Health(ctx context.Context) (*oas.Health, error)

	// Movies
	GetMovies(ctx context.Context, limit, offset int) (movies []oas.Movie, total int, err error)
	GetMovieById(ctx context.Context, movieID string) (*oas.Movie, error)
	GetMovieStreams(ctx context.Context, movieID string) (streams []oas.Stream, total int, err error)
	GetMovieStreamFile(ctx context.Context, movieID, streamFile string) (r io.ReadCloser, mimeType string, err error) // mimeType so handler can set Content-Type
	GetMovieSubtitles(ctx context.Context, movieID string) (subtitles []oas.Subtitle, total int, err error)
	GetMovieSubtitleFile(ctx context.Context, movieID, subtitleFile string) (r io.ReadCloser, mimeType string, err error)
	GetMoviePoster(ctx context.Context, movieID string) (io.ReadCloser, string, error)
	GetMovieBackdrop(ctx context.Context, movieID string) (io.ReadCloser, string, error)

	// Series
	GetSeries(ctx context.Context, limit, offset int) (series []oas.Series, total int, err error)
	GetSeriesByID(ctx context.Context, seriesID string) (*oas.Series, error)
	GetSeriesPoster(ctx context.Context, seriesID string) (io.ReadCloser, string, error)
	GetSeriesBackdrop(ctx context.Context, seriesID string) (io.ReadCloser, string, error)

	// Seasons
	GetSeasons(ctx context.Context, seriesID string, limit, offset int) (seasons []oas.Season, total int, err error)
	GetSeasonById(ctx context.Context, seriesID, seasonID string) (*oas.Season, error)
	GetSeasonPoster(ctx context.Context, seriesID, seasonID string) (io.ReadCloser, string, error)
	GetSeasonBackdrop(ctx context.Context, seriesID, seasonID string) (io.ReadCloser, string, error)

	// Episodes
	GetEpisodes(ctx context.Context, seriesID, seasonID string, limit, offset int) (episodes []oas.Episode, total int, err error)
	GetEpisodeById(ctx context.Context, seriesID, seasonID, episodeID string) (*oas.Episode, error)
	GetEpisodeStreams(ctx context.Context, seriesID, seasonID, episodeID string) (streams []oas.Stream, total int, err error)
	GetEpisodeStreamFile(ctx context.Context, seriesID, seasonID, episodeID, streamFile string) (r io.ReadCloser, mimeType string, err error)
	GetEpisodeSubtitles(ctx context.Context, seriesID, seasonID, episodeID string) (subtitles []oas.Subtitle, total int, err error)
	GetEpisodeSubtitleFile(ctx context.Context, seriesID, seasonID, episodeID, subtitleFile string) (r io.ReadCloser, mimeType string, err error)
	GetEpisodeThumbnail(ctx context.Context, seriesID, seasonID, episodeID string) (io.ReadCloser, string, error)

	// Search
	Search(ctx context.Context, query string, limit, offset int) (results []oas.SearchResultItem, total int, err error)
}
