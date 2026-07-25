package provider

import (
	"context"
	"io"

	"github.com/nem-git/abcmovies/internal/oas"
)

// Compile-time check for Provider.
var _ Provider = (*UnimplementedProvider)(nil)

type UnimplementedProvider struct{}

func (p *UnimplementedProvider) Tag() string {
	return ""
}

func (p *UnimplementedProvider) Service(context.Context) (*oas.Service, error) {
	return nil, ErrNotSupported
}

func (p *UnimplementedProvider) Health(context.Context) (*oas.Health, error) {
	return nil, ErrNotSupported
}

// Movies
func (p *UnimplementedProvider) GetMovies(context.Context, int, int) ([]oas.Movie, int, error) {
	return nil, 0, ErrNotSupported
}

func (p *UnimplementedProvider) GetMovieById(context.Context, string) (*oas.Movie, error) {
	return nil, ErrNotSupported
}

func (p *UnimplementedProvider) GetMovieStreams(context.Context, string) ([]oas.Stream, int, error) {
	return nil, 0, ErrNotSupported
}

func (p *UnimplementedProvider) GetMovieStreamFile(context.Context, string, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

func (p *UnimplementedProvider) GetMovieSubtitles(context.Context, string) ([]oas.Subtitle, int, error) {
	return nil, 0, ErrNotSupported
}

func (p *UnimplementedProvider) GetMovieSubtitleFile(context.Context, string, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

func (p *UnimplementedProvider) GetMoviePoster(context.Context, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

func (p *UnimplementedProvider) GetMovieBackdrop(context.Context, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

// Series
func (p *UnimplementedProvider) GetSeries(context.Context, int, int) ([]oas.Series, int, error) {
	return nil, 0, ErrNotSupported
}

func (p *UnimplementedProvider) GetSeriesByID(context.Context, string) (*oas.Series, error) {
	return nil, ErrNotSupported
}

func (p *UnimplementedProvider) GetSeriesPoster(context.Context, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

func (p *UnimplementedProvider) GetSeriesBackdrop(context.Context, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

// Seasons
func (p *UnimplementedProvider) GetSeasons(context.Context, string, int, int) ([]oas.Season, int, error) {
	return nil, 0, ErrNotSupported
}

func (p *UnimplementedProvider) GetSeasonById(context.Context, string, string) (*oas.Season, error) {
	return nil, ErrNotSupported
}

func (p *UnimplementedProvider) GetSeasonPoster(context.Context, string, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

func (p *UnimplementedProvider) GetSeasonBackdrop(context.Context, string, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

// Episodes
func (p *UnimplementedProvider) GetEpisodes(context.Context, string, string, int, int) ([]oas.Episode, int, error) {
	return nil, 0, ErrNotSupported
}

func (p *UnimplementedProvider) GetEpisodeById(context.Context, string, string, string) (*oas.Episode, error) {
	return nil, ErrNotSupported
}

func (p *UnimplementedProvider) GetEpisodeStreams(context.Context, string, string, string) ([]oas.Stream, int, error) {
	return nil, 0, ErrNotSupported
}

func (p *UnimplementedProvider) GetEpisodeStreamFile(context.Context, string, string, string, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

func (p *UnimplementedProvider) GetEpisodeSubtitles(context.Context, string, string, string) ([]oas.Subtitle, int, error) {
	return nil, 0, ErrNotSupported
}

func (p *UnimplementedProvider) GetEpisodeSubtitleFile(context.Context, string, string, string, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

func (p *UnimplementedProvider) GetEpisodeThumbnail(context.Context, string, string, string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotSupported
}

// Search
func (p *UnimplementedProvider) Search(context.Context, string, int, int) ([]oas.SearchResultItem, int, error) {
	return nil, 0, ErrNotSupported
}
