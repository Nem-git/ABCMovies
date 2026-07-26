package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/stream"
)

type Config struct {
	Tag     string
	BaseURL string
	Timeout time.Duration
}

type pageResponse[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type Provider struct {
	provider.UnimplementedProvider
	tag    string
	base   string
	client *http.Client
}

func New(cfg Config) *Provider {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Provider{
		tag:  cfg.Tag,
		base: cfg.BaseURL,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (p *Provider) Tag() string {
	return p.tag
}

func (p *Provider) Service(ctx context.Context) (*oas.Service, error) {
	return getItem[oas.Service](p, ctx, "/service")
}

func (p *Provider) Health(ctx context.Context) (*oas.Health, error) {
	return getItem[oas.Health](p, ctx, "/health")
}

func (p *Provider) GetMovies(ctx context.Context, limit, offset int) ([]oas.Movie, int, error) {
	return getPage[oas.Movie](p, ctx, fmt.Sprintf("/movies?limit=%d&offset=%d", limit, offset))
}

func (p *Provider) GetMovieById(ctx context.Context, movieID string) (*oas.Movie, error) {
	return getItem[oas.Movie](p, ctx, fmt.Sprintf("/movies/%s", url.PathEscape(movieID)))
}

func (p *Provider) GetMovieStreams(ctx context.Context, movieID string) ([]oas.Stream, int, error) {
	return getPage[oas.Stream](p, ctx, fmt.Sprintf("/movies/%s/streams", url.PathEscape(movieID)))
}

func (p *Provider) GetMovieStreamLocator(ctx context.Context, movieID, streamFile string) (*stream.Locator, error) {
	rc, mime, err := getRaw(p, ctx, fmt.Sprintf("/movies/%s/streams/%s", url.PathEscape(movieID), url.PathEscape(streamFile)))
	if err != nil {
		return nil, err
	}
	return &stream.Locator{
		URL:            p.base + fmt.Sprintf("/movies/%s/streams/%s", url.PathEscape(movieID), url.PathEscape(streamFile)),
		EncodingFormat: mime,
		Data:           rc,
	}, nil
}

func (p *Provider) GetMovieSubtitles(ctx context.Context, movieID string) ([]oas.Subtitle, int, error) {
	return getPage[oas.Subtitle](p, ctx, fmt.Sprintf("/movies/%s/subtitles", url.PathEscape(movieID)))
}

func (p *Provider) GetMovieSubtitleFile(ctx context.Context, movieID, subtitleFile string) (io.ReadCloser, string, error) {
	return getRaw(p, ctx, fmt.Sprintf("/movies/%s/subtitles/%s", url.PathEscape(movieID), url.PathEscape(subtitleFile)))
}

func (p *Provider) GetMoviePoster(ctx context.Context, movieID string) (io.ReadCloser, string, error) {
	return getRaw(p, ctx, fmt.Sprintf("/movies/%s/poster", url.PathEscape(movieID)))
}

func (p *Provider) GetMovieBackdrop(ctx context.Context, movieID string) (io.ReadCloser, string, error) {
	return getRaw(p, ctx, fmt.Sprintf("/movies/%s/backdrop", url.PathEscape(movieID)))
}

func (p *Provider) GetSeries(ctx context.Context, limit, offset int) ([]oas.Series, int, error) {
	return getPage[oas.Series](p, ctx, fmt.Sprintf("/series?limit=%d&offset=%d", limit, offset))
}

func (p *Provider) GetSeriesByID(ctx context.Context, seriesID string) (*oas.Series, error) {
	return getItem[oas.Series](p, ctx, fmt.Sprintf("/series/%s", url.PathEscape(seriesID)))
}

func (p *Provider) GetSeriesPoster(ctx context.Context, seriesID string) (io.ReadCloser, string, error) {
	return getRaw(p, ctx, fmt.Sprintf("/series/%s/poster", url.PathEscape(seriesID)))
}

func (p *Provider) GetSeriesBackdrop(ctx context.Context, seriesID string) (io.ReadCloser, string, error) {
	return getRaw(p, ctx, fmt.Sprintf("/series/%s/backdrop", url.PathEscape(seriesID)))
}

func (p *Provider) GetSeasons(ctx context.Context, seriesID string, limit, offset int) ([]oas.Season, int, error) {
	return getPage[oas.Season](p, ctx, fmt.Sprintf("/series/%s/seasons?limit=%d&offset=%d", url.PathEscape(seriesID), limit, offset))
}

func (p *Provider) GetSeasonById(ctx context.Context, seriesID, seasonID string) (*oas.Season, error) {
	return getItem[oas.Season](p, ctx, fmt.Sprintf("/series/%s/seasons/%s", url.PathEscape(seriesID), url.PathEscape(seasonID)))
}

func (p *Provider) GetSeasonPoster(ctx context.Context, seriesID, seasonID string) (io.ReadCloser, string, error) {
	return getRaw(p, ctx, fmt.Sprintf("/series/%s/seasons/%s/poster", url.PathEscape(seriesID), url.PathEscape(seasonID)))
}

func (p *Provider) GetSeasonBackdrop(ctx context.Context, seriesID, seasonID string) (io.ReadCloser, string, error) {
	return getRaw(p, ctx, fmt.Sprintf("/series/%s/seasons/%s/backdrop", url.PathEscape(seriesID), url.PathEscape(seasonID)))
}

func (p *Provider) GetEpisodes(ctx context.Context, seriesID, seasonID string, limit, offset int) ([]oas.Episode, int, error) {
	return getPage[oas.Episode](p, ctx, fmt.Sprintf("/series/%s/seasons/%s/episodes?limit=%d&offset=%d",
		url.PathEscape(seriesID), url.PathEscape(seasonID), limit, offset))
}

func (p *Provider) GetEpisodeById(ctx context.Context, seriesID, seasonID, episodeID string) (*oas.Episode, error) {
	return getItem[oas.Episode](p, ctx, fmt.Sprintf("/series/%s/seasons/%s/episodes/%s",
		url.PathEscape(seriesID), url.PathEscape(seasonID), url.PathEscape(episodeID)))
}

func (p *Provider) GetEpisodeStreams(ctx context.Context, seriesID, seasonID, episodeID string) ([]oas.Stream, int, error) {
	return getPage[oas.Stream](p, ctx, fmt.Sprintf("/series/%s/seasons/%s/episodes/%s/streams",
		url.PathEscape(seriesID), url.PathEscape(seasonID), url.PathEscape(episodeID)))
}

func (p *Provider) GetEpisodeStreamLocator(ctx context.Context, seriesID, seasonID, episodeID, streamFile string) (*stream.Locator, error) {
	rc, mime, err := getRaw(p, ctx, fmt.Sprintf("/series/%s/seasons/%s/episodes/%s/streams/%s",
		url.PathEscape(seriesID), url.PathEscape(seasonID), url.PathEscape(episodeID), url.PathEscape(streamFile)))
	if err != nil {
		return nil, err
	}
	return &stream.Locator{
		URL:            p.base + fmt.Sprintf("/series/%s/seasons/%s/episodes/%s/streams/%s", url.PathEscape(seriesID), url.PathEscape(seasonID), url.PathEscape(episodeID), url.PathEscape(streamFile)),
		EncodingFormat: mime,
		Data:           rc,
	}, nil
}

func (p *Provider) GetEpisodeSubtitles(ctx context.Context, seriesID, seasonID, episodeID string) ([]oas.Subtitle, int, error) {
	return getPage[oas.Subtitle](p, ctx, fmt.Sprintf("/series/%s/seasons/%s/episodes/%s/subtitles",
		url.PathEscape(seriesID), url.PathEscape(seasonID), url.PathEscape(episodeID)))
}

func (p *Provider) GetEpisodeSubtitleFile(ctx context.Context, seriesID, seasonID, episodeID, subtitleFile string) (io.ReadCloser, string, error) {
	return getRaw(p, ctx, fmt.Sprintf("/series/%s/seasons/%s/episodes/%s/subtitles/%s",
		url.PathEscape(seriesID), url.PathEscape(seasonID), url.PathEscape(episodeID), url.PathEscape(subtitleFile)))
}

func (p *Provider) GetEpisodeThumbnail(ctx context.Context, seriesID, seasonID, episodeID string) (io.ReadCloser, string, error) {
	return getRaw(p, ctx, fmt.Sprintf("/series/%s/seasons/%s/episodes/%s/thumbnail",
		url.PathEscape(seriesID), url.PathEscape(seasonID), url.PathEscape(episodeID)))
}

func (p *Provider) Search(ctx context.Context, query string, limit, offset int) ([]oas.SearchResultItem, int, error) {
	return getPage[oas.SearchResultItem](p, ctx, fmt.Sprintf("/search?q=%s&limit=%d&offset=%d", url.QueryEscape(query), limit, offset))
}

func (p *Provider) doRequest(ctx context.Context, path string) (*http.Response, error) {
	u := p.base + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}

func (p *Provider) getJSON(ctx context.Context, path string, dest any) error {
	resp, err := p.doRequest(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNotImplemented {
		return provider.ErrNotSupported
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}

func getItem[T any](p *Provider, ctx context.Context, path string) (*T, error) {
	var item T
	if err := p.getJSON(ctx, path, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func getPage[T any](p *Provider, ctx context.Context, path string) ([]T, int, error) {
	var page pageResponse[T]
	if err := p.getJSON(ctx, path, &page); err != nil {
		return nil, 0, err
	}
	return page.Items, page.Total, nil
}

func getRaw(p *Provider, ctx context.Context, path string) (io.ReadCloser, string, error) {
	resp, err := p.doRequest(ctx, path)
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNotImplemented {
		resp.Body.Close()
		return nil, "", provider.ErrNotSupported
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// doesn't defer resp.Body.Close() to be able to read body in the program
	// without passing the whole data
	return resp.Body, resp.Header.Get("Content-Type"), nil
}
