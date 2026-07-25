package stub

import (
	"bytes"
	"context"
	"io"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
)

type Config struct {
	Tag     string
	Service *oas.Service
	Health  *oas.Health
	Error   error

	Movies    []oas.Movie
	Series    []oas.Series
	Seasons   []oas.Season
	Episodes  []oas.Episode
	Streams   []oas.Stream
	Subtitles []oas.Subtitle
	Search    []oas.SearchResultItem

	MoviePosterData      []byte
	MovieBackdropData    []byte
	SeriesPosterData     []byte
	SeriesBackdropData   []byte
	SeasonPosterData     []byte
	SeasonBackdropData   []byte
	EpisodeThumbnailData []byte
	ImageMIME            string
	StreamFileData       []byte
	StreamFileMIME       string
	SubtitleFileData     []byte
	SubtitleFileMIME     string
}

type Provider struct {
	provider.UnimplementedProvider
	cfg Config
}

func New(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Tag() string {
	return p.cfg.Tag
}

func (p *Provider) Service(ctx context.Context) (*oas.Service, error) {
	if p.cfg.Error != nil {
		return nil, p.cfg.Error
	}
	if p.cfg.Service == nil {
		return p.UnimplementedProvider.Service(ctx)
	}
	return p.cfg.Service, nil
}

func (p *Provider) Health(ctx context.Context) (*oas.Health, error) {
	if p.cfg.Error != nil {
		return nil, p.cfg.Error
	}
	if p.cfg.Health == nil {
		return p.UnimplementedProvider.Health(ctx)
	}
	return p.cfg.Health, nil
}

func (p *Provider) GetMovies(ctx context.Context, limit, offset int) ([]oas.Movie, int, error) {
	if p.cfg.Error != nil {
		return nil, 0, p.cfg.Error
	}
	if p.cfg.Movies == nil {
		return p.UnimplementedProvider.GetMovies(ctx, limit, offset)
	}
	items, total := Paginate(p.cfg.Movies, limit, offset)
	return items, total, nil
}

func (p *Provider) GetMovieById(ctx context.Context, movieID string) (*oas.Movie, error) {
	if p.cfg.Error != nil {
		return nil, p.cfg.Error
	}
	if p.cfg.Movies == nil {
		return p.UnimplementedProvider.GetMovieById(ctx, movieID)
	}
	return findItem(p.cfg.Movies, func(m oas.Movie) bool { return m.GetID() == movieID })
}

func (p *Provider) GetMovieStreams(ctx context.Context, movieID string) ([]oas.Stream, int, error) {
	if p.cfg.Error != nil {
		return nil, 0, p.cfg.Error
	}
	if p.cfg.Streams == nil {
		return p.UnimplementedProvider.GetMovieStreams(ctx, movieID)
	}
	return p.cfg.Streams, len(p.cfg.Streams), nil
}

func (p *Provider) GetMovieStreamFile(ctx context.Context, movieID, streamFile string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.StreamFileData == nil {
		return p.UnimplementedProvider.GetMovieStreamFile(ctx, movieID, streamFile)
	}
	return p.file(p.cfg.StreamFileData, p.cfg.StreamFileMIME)
}

func (p *Provider) GetMovieSubtitles(ctx context.Context, movieID string) ([]oas.Subtitle, int, error) {
	if p.cfg.Error != nil {
		return nil, 0, p.cfg.Error
	}
	if p.cfg.Subtitles == nil {
		return p.UnimplementedProvider.GetMovieSubtitles(ctx, movieID)
	}
	return p.cfg.Subtitles, len(p.cfg.Subtitles), nil
}

func (p *Provider) GetMovieSubtitleFile(ctx context.Context, movieID, subtitleFile string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.SubtitleFileData == nil {
		return p.UnimplementedProvider.GetMovieSubtitleFile(ctx, movieID, subtitleFile)
	}
	return p.file(p.cfg.SubtitleFileData, p.cfg.SubtitleFileMIME)
}

func (p *Provider) GetMoviePoster(ctx context.Context, movieID string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.MoviePosterData == nil {
		return p.UnimplementedProvider.GetMoviePoster(ctx, movieID)
	}
	return p.image(p.cfg.MoviePosterData)
}

func (p *Provider) GetMovieBackdrop(ctx context.Context, movieID string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.MovieBackdropData == nil {
		return p.UnimplementedProvider.GetMovieBackdrop(ctx, movieID)
	}
	return p.image(p.cfg.MovieBackdropData)
}

func (p *Provider) GetSeries(ctx context.Context, limit, offset int) ([]oas.Series, int, error) {
	if p.cfg.Error != nil {
		return nil, 0, p.cfg.Error
	}
	if p.cfg.Series == nil {
		return p.UnimplementedProvider.GetSeries(ctx, limit, offset)
	}
	items, total := Paginate(p.cfg.Series, limit, offset)
	return items, total, nil
}

func (p *Provider) GetSeriesByID(ctx context.Context, seriesID string) (*oas.Series, error) {
	if p.cfg.Error != nil {
		return nil, p.cfg.Error
	}
	if p.cfg.Series == nil {
		return p.UnimplementedProvider.GetSeriesByID(ctx, seriesID)
	}
	return findItem(p.cfg.Series, func(s oas.Series) bool { return s.GetID() == seriesID })
}

func (p *Provider) GetSeriesPoster(ctx context.Context, seriesID string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.SeriesPosterData == nil {
		return p.UnimplementedProvider.GetSeriesPoster(ctx, seriesID)
	}
	return p.image(p.cfg.SeriesPosterData)
}

func (p *Provider) GetSeriesBackdrop(ctx context.Context, seriesID string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.SeriesBackdropData == nil {
		return p.UnimplementedProvider.GetSeriesBackdrop(ctx, seriesID)
	}
	return p.image(p.cfg.SeriesBackdropData)
}

func (p *Provider) GetSeasons(ctx context.Context, seriesID string, limit, offset int) ([]oas.Season, int, error) {
	if p.cfg.Error != nil {
		return nil, 0, p.cfg.Error
	}
	if p.cfg.Seasons == nil {
		return p.UnimplementedProvider.GetSeasons(ctx, seriesID, limit, offset)
	}
	items, total := Paginate(p.cfg.Seasons, limit, offset)
	return items, total, nil
}

func (p *Provider) GetSeasonById(ctx context.Context, seriesID, seasonID string) (*oas.Season, error) {
	if p.cfg.Error != nil {
		return nil, p.cfg.Error
	}
	if p.cfg.Seasons == nil {
		return p.UnimplementedProvider.GetSeasonById(ctx, seriesID, seasonID)
	}
	return findItem(p.cfg.Seasons, func(s oas.Season) bool { return s.GetID() == seasonID })
}

func (p *Provider) GetSeasonPoster(ctx context.Context, seriesID, seasonID string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.SeasonPosterData == nil {
		return p.UnimplementedProvider.GetSeasonPoster(ctx, seriesID, seasonID)
	}
	return p.image(p.cfg.SeasonPosterData)
}

func (p *Provider) GetSeasonBackdrop(ctx context.Context, seriesID, seasonID string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.SeasonBackdropData == nil {
		return p.UnimplementedProvider.GetSeasonBackdrop(ctx, seriesID, seasonID)
	}
	return p.image(p.cfg.SeasonBackdropData)
}

func (p *Provider) GetEpisodes(ctx context.Context, seriesID, seasonID string, limit, offset int) ([]oas.Episode, int, error) {
	if p.cfg.Error != nil {
		return nil, 0, p.cfg.Error
	}
	if p.cfg.Episodes == nil {
		return p.UnimplementedProvider.GetEpisodes(ctx, seriesID, seasonID, limit, offset)
	}
	items, total := Paginate(p.cfg.Episodes, limit, offset)
	return items, total, nil
}

func (p *Provider) GetEpisodeById(ctx context.Context, seriesID, seasonID, episodeID string) (*oas.Episode, error) {
	if p.cfg.Error != nil {
		return nil, p.cfg.Error
	}
	if p.cfg.Episodes == nil {
		return p.UnimplementedProvider.GetEpisodeById(ctx, seriesID, seasonID, episodeID)
	}
	return findItem(p.cfg.Episodes, func(e oas.Episode) bool { return e.GetID() == episodeID })
}

func (p *Provider) GetEpisodeStreams(ctx context.Context, seriesID, seasonID, episodeID string) ([]oas.Stream, int, error) {
	if p.cfg.Error != nil {
		return nil, 0, p.cfg.Error
	}
	if p.cfg.Streams == nil {
		return p.UnimplementedProvider.GetEpisodeStreams(ctx, seriesID, seasonID, episodeID)
	}
	return p.cfg.Streams, len(p.cfg.Streams), nil
}

func (p *Provider) GetEpisodeStreamFile(ctx context.Context, seriesID, seasonID, episodeID, streamFile string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.StreamFileData == nil {
		return p.UnimplementedProvider.GetEpisodeStreamFile(ctx, seriesID, seasonID, episodeID, streamFile)
	}
	return p.file(p.cfg.StreamFileData, p.cfg.StreamFileMIME)
}

func (p *Provider) GetEpisodeSubtitles(ctx context.Context, seriesID, seasonID, episodeID string) ([]oas.Subtitle, int, error) {
	if p.cfg.Error != nil {
		return nil, 0, p.cfg.Error
	}
	if p.cfg.Subtitles == nil {
		return p.UnimplementedProvider.GetEpisodeSubtitles(ctx, seriesID, seasonID, episodeID)
	}
	return p.cfg.Subtitles, len(p.cfg.Subtitles), nil
}

func (p *Provider) GetEpisodeSubtitleFile(ctx context.Context, seriesID, seasonID, episodeID, subtitleFile string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.SubtitleFileData == nil {
		return p.UnimplementedProvider.GetEpisodeSubtitleFile(ctx, seriesID, seasonID, episodeID, subtitleFile)
	}
	return p.file(p.cfg.SubtitleFileData, p.cfg.SubtitleFileMIME)
}

func (p *Provider) GetEpisodeThumbnail(ctx context.Context, seriesID, seasonID, episodeID string) (io.ReadCloser, string, error) {
	if p.cfg.Error != nil {
		return nil, "", p.cfg.Error
	}
	if p.cfg.EpisodeThumbnailData == nil {
		return p.UnimplementedProvider.GetEpisodeThumbnail(ctx, seriesID, seasonID, episodeID)
	}
	return p.image(p.cfg.EpisodeThumbnailData)
}

func (p *Provider) Search(ctx context.Context, query string, limit, offset int) ([]oas.SearchResultItem, int, error) {
	if p.cfg.Error != nil {
		return nil, 0, p.cfg.Error
	}
	if p.cfg.Search == nil {
		return p.UnimplementedProvider.Search(ctx, query, limit, offset)
	}
	items, total := Paginate(p.cfg.Search, limit, offset)
	return items, total, nil
}

func Paginate[T any](items []T, limit, offset int) ([]T, int) {
	n := len(items)
	if n == 0 || offset >= n || limit <= 0 {
		return nil, n
	}
	end := offset + limit
	if end > n {
		end = n
	}
	return items[offset:end], n
}

func findItem[T any](items []T, fn func(T) bool) (*T, error) {
	for i := range items {
		if fn(items[i]) {
			return &items[i], nil
		}
	}
	return nil, provider.ErrNotSupported
}

func (p *Provider) file(data []byte, mime string) (io.ReadCloser, string, error) {
	if mime == "" {
		mime = "application/octet-stream"
	}
	return io.NopCloser(bytes.NewReader(data)), mime, nil
}

func (p *Provider) image(data []byte) (io.ReadCloser, string, error) {
	mime := p.cfg.ImageMIME
	if mime == "" {
		mime = "image/png"
	}
	return io.NopCloser(bytes.NewReader(data)), mime, nil
}
