package handler

import (
	"context"
	"fmt"
	"io"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/registry"
	"github.com/nem-git/abcmovies/internal/search"
)

var _ oas.Handler = (*Handler)(nil)

const defaultLimit = 20

type Handler struct {
	oas.UnimplementedHandler
	registry *registry.Registry
}

func New(r *registry.Registry) *Handler {
	return &Handler{registry: r}
}

func (h *Handler) providerOrError(tag string) (provider.Provider, error) {
	p, err := h.registry.Get(tag)
	if err != nil {
		return nil, &oas.ErrorStatusCode{
			StatusCode: 404,
			Response:   oas.Error{Code: "NOT_FOUND", Message: "streaming service not found"},
		}
	}
	return p, nil
}

func providerErrorMessage(msg string) *oas.ErrorStatusCode {
	return &oas.ErrorStatusCode{
		StatusCode: 502,
		Response:   oas.Error{Code: "PROVIDER_ERROR", Message: msg},
	}
}

func providerError(err error) *oas.ErrorStatusCode {
	return providerErrorMessage(err.Error())
}

// --- Global ---

func (h *Handler) GetHealth(ctx context.Context) (*oas.Health, error) {
	for _, p := range h.registry.All() {
		health, err := p.Health(ctx)
		if err != nil || health.Status != oas.HealthStatusOk {
			return &oas.Health{Status: oas.HealthStatusDegraded}, nil
		}
	}
	return &oas.Health{Status: oas.HealthStatusOk}, nil
}

func (h *Handler) GlobalSearch(ctx context.Context, params oas.GlobalSearchParams) (oas.GlobalSearchRes, error) {
	limit := params.Limit.Or(defaultLimit)
	offset := params.Offset.Or(0)

	var allResults []search.Result
	for _, p := range h.registry.All() {
		items, _, err := p.Search(ctx, params.Q, limit, offset)
		if err != nil {
			continue
		}
		for _, item := range items {
			allResults = append(allResults, search.Result{Tag: p.Tag(), Item: item})
		}
	}

	if len(params.Type) > 0 {
		types := make([]string, len(params.Type))
		for i, t := range params.Type {
			types[i] = string(t)
		}
		allResults = search.FilterByType(allResults, types)
	}

	allResults = search.ScoreAndSort(params.Q, allResults)

	items := make([]oas.SearchResultItem, len(allResults))
	for i, r := range allResults {
		items[i] = r.Item
	}

	return &oas.PageSearchResult{
		Items:  items,
		Total:  len(items),
		Limit:  limit,
		Offset: offset,
	}, nil
}

// --- GetServices ---

func (h *Handler) GetServices(ctx context.Context, params oas.GetServicesParams) (*oas.PageService, error) {
	providers := h.registry.All()

	services := make([]oas.Service, 0, len(providers))

	for _, p := range providers {
		service, err := p.Service(ctx)
		if err != nil {
			return nil, providerErrorMessage("upstreams unavailable")
		}
		services = append(services, *service)
	}

	if params.Offset.Value > len(services) {
		return nil, &oas.ErrorStatusCode{
			StatusCode: 400,
			Response:   oas.Error{Code: "BAD_REQUEST", Message: "offset was over total services amount"},
		}
	}

	end := min(params.Offset.Value+params.Limit.Value, len(services))

	return &oas.PageService{
		Items:  services[params.Offset.Value:end],
		Total:  len(services),
		Limit:  params.Limit.Value,
		Offset: params.Offset.Value,
	}, nil
}

// --- GetServiceByTag ---

func (h *Handler) GetServiceByTag(ctx context.Context, params oas.GetServiceByTagParams) (oas.GetServiceByTagRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	svc, err := p.Service(ctx)
	if err != nil {
		return nil, providerError(err)
	}
	return svc, nil
}

// --- Movies ---

func (h *Handler) GetMovies(ctx context.Context, params oas.GetMoviesParams) (oas.GetMoviesRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(defaultLimit)
	offset := params.Offset.Or(0)
	items, total, err := p.GetMovies(ctx, limit, offset)
	if err != nil {
		return nil, providerError(err)
	}
	return &oas.PageMovie{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (h *Handler) GetMovieById(ctx context.Context, params oas.GetMovieByIdParams) (oas.GetMovieByIdRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	movie, err := p.GetMovieById(ctx, params.MovieId)
	if err != nil {
		return nil, providerError(err)
	}
	return movie, nil
}

func (h *Handler) GetMovieStreams(ctx context.Context, params oas.GetMovieStreamsParams) (oas.GetMovieStreamsRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	items, total, err := p.GetMovieStreams(ctx, params.MovieId)
	if err != nil {
		return nil, providerError(err)
	}
	return &oas.PageStream{Items: items, Total: total, Limit: params.Limit.Or(defaultLimit), Offset: params.Offset.Or(0)}, nil
}

func (h *Handler) GetMoviePoster(ctx context.Context, params oas.GetMoviePosterParams) (oas.GetMoviePosterRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetMoviePoster(ctx, params.MovieId)
	if err != nil {
		return nil, providerError(err)
	}
	return moviePosterResponse(reader, mimeType)
}

func (h *Handler) GetMovieBackdrop(ctx context.Context, params oas.GetMovieBackdropParams) (oas.GetMovieBackdropRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetMovieBackdrop(ctx, params.MovieId)
	if err != nil {
		return nil, providerError(err)
	}
	return movieBackdropResponse(reader, mimeType)
}

func (h *Handler) GetMovieSubtitles(ctx context.Context, params oas.GetMovieSubtitlesParams) (oas.GetMovieSubtitlesRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	items, total, err := p.GetMovieSubtitles(ctx, params.MovieId)
	if err != nil {
		return nil, providerError(err)
	}
	return &oas.PageSubtitle{Items: items, Total: total, Limit: params.Limit.Or(defaultLimit), Offset: params.Offset.Or(0)}, nil
}

// --- Series ---

func (h *Handler) GetSeries(ctx context.Context, params oas.GetSeriesParams) (oas.GetSeriesRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(defaultLimit)
	offset := params.Offset.Or(0)
	items, total, err := p.GetSeries(ctx, limit, offset)
	if err != nil {
		return nil, providerError(err)
	}
	return &oas.PageSeries{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (h *Handler) GetSeriesById(ctx context.Context, params oas.GetSeriesByIdParams) (oas.GetSeriesByIdRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	series, err := p.GetSeriesByID(ctx, params.SeriesId)
	if err != nil {
		return nil, providerError(err)
	}
	return series, nil
}

func (h *Handler) GetSeriesPoster(ctx context.Context, params oas.GetSeriesPosterParams) (oas.GetSeriesPosterRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetSeriesPoster(ctx, params.SeriesId)
	if err != nil {
		return nil, providerError(err)
	}
	return seriesPosterResponse(reader, mimeType)
}

func (h *Handler) GetSeriesBackdrop(ctx context.Context, params oas.GetSeriesBackdropParams) (oas.GetSeriesBackdropRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetSeriesBackdrop(ctx, params.SeriesId)
	if err != nil {
		return nil, providerError(err)
	}
	return seriesBackdropResponse(reader, mimeType)
}

// --- Seasons ---

func (h *Handler) GetSeasons(ctx context.Context, params oas.GetSeasonsParams) (oas.GetSeasonsRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(defaultLimit)
	offset := params.Offset.Or(0)
	items, total, err := p.GetSeasons(ctx, params.SeriesId, limit, offset)
	if err != nil {
		return nil, providerError(err)
	}
	return &oas.PageSeason{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (h *Handler) GetSeasonById(ctx context.Context, params oas.GetSeasonByIdParams) (oas.GetSeasonByIdRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	season, err := p.GetSeasonById(ctx, params.SeriesId, params.SeasonId)
	if err != nil {
		return nil, providerError(err)
	}
	return season, nil
}

func (h *Handler) GetSeasonPoster(ctx context.Context, params oas.GetSeasonPosterParams) (oas.GetSeasonPosterRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetSeasonPoster(ctx, params.SeriesId, params.SeasonId)
	if err != nil {
		return nil, providerError(err)
	}
	return seasonPosterResponse(reader, mimeType)
}

func (h *Handler) GetSeasonBackdrop(ctx context.Context, params oas.GetSeasonBackdropParams) (oas.GetSeasonBackdropRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetSeasonBackdrop(ctx, params.SeriesId, params.SeasonId)
	if err != nil {
		return nil, providerError(err)
	}
	return seasonBackdropResponse(reader, mimeType)
}

// --- Episodes ---

func (h *Handler) GetEpisodes(ctx context.Context, params oas.GetEpisodesParams) (oas.GetEpisodesRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	limit := params.Limit.Or(defaultLimit)
	offset := params.Offset.Or(0)
	items, total, err := p.GetEpisodes(ctx, params.SeriesId, params.SeasonId, limit, offset)
	if err != nil {
		return nil, providerError(err)
	}
	return &oas.PageEpisode{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (h *Handler) GetEpisodeById(ctx context.Context, params oas.GetEpisodeByIdParams) (oas.GetEpisodeByIdRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	episode, err := p.GetEpisodeById(ctx, params.SeriesId, params.SeasonId, params.EpisodeId)
	if err != nil {
		return nil, providerError(err)
	}
	return episode, nil
}

func (h *Handler) GetEpisodeStreams(ctx context.Context, params oas.GetEpisodeStreamsParams) (oas.GetEpisodeStreamsRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	items, total, err := p.GetEpisodeStreams(ctx, params.SeriesId, params.SeasonId, params.EpisodeId)
	if err != nil {
		return nil, providerError(err)
	}
	return &oas.PageStream{Items: items, Total: total, Limit: params.Limit.Or(defaultLimit), Offset: params.Offset.Or(0)}, nil
}

func (h *Handler) GetEpisodeSubtitles(ctx context.Context, params oas.GetEpisodeSubtitlesParams) (oas.GetEpisodeSubtitlesRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	items, total, err := p.GetEpisodeSubtitles(ctx, params.SeriesId, params.SeasonId, params.EpisodeId)
	if err != nil {
		return nil, providerError(err)
	}
	return &oas.PageSubtitle{Items: items, Total: total, Limit: params.Limit.Or(defaultLimit), Offset: params.Offset.Or(0)}, nil
}

func (h *Handler) GetEpisodeThumbnail(ctx context.Context, params oas.GetEpisodeThumbnailParams) (oas.GetEpisodeThumbnailRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetEpisodeThumbnail(ctx, params.SeriesId, params.SeasonId, params.EpisodeId)
	if err != nil {
		return nil, providerError(err)
	}
	return episodeThumbnailResponse(reader, mimeType)
}

// --- Stream file methods ---

func (h *Handler) GetMovieStreamFile(ctx context.Context, params oas.GetMovieStreamFileParams) (oas.GetMovieStreamFileRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetMovieStreamFile(ctx, params.MovieId, params.StreamFile)
	if err != nil {
		return nil, providerError(err)
	}
	return movieStreamFileResponse(reader, mimeType)
}

func movieStreamFileResponse(r io.Reader, mimeType string) (oas.GetMovieStreamFileRes, error) {
	switch mimeType {
	case "application/dash+xml":
		return &oas.GetMovieStreamFileOKApplicationDashXML{Data: r}, nil
	case "application/vnd.apple.mpegurl":
		return &oas.GetMovieStreamFileOKApplicationVndAppleMpegurl{Data: r}, nil
	case "video/mp4":
		return &oas.GetMovieStreamFileOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported stream mime type: %s", mimeType)
	}
}

func (h *Handler) GetEpisodeStreamFile(ctx context.Context, params oas.GetEpisodeStreamFileParams) (oas.GetEpisodeStreamFileRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetEpisodeStreamFile(ctx, params.SeriesId, params.SeasonId, params.EpisodeId, params.StreamFile)
	if err != nil {
		return nil, providerError(err)
	}
	return episodeStreamFileResponse(reader, mimeType)
}

func episodeStreamFileResponse(r io.Reader, mimeType string) (oas.GetEpisodeStreamFileRes, error) {
	switch mimeType {
	case "application/dash+xml":
		return &oas.GetEpisodeStreamFileOKApplicationDashXML{Data: r}, nil
	case "application/vnd.apple.mpegurl":
		return &oas.GetEpisodeStreamFileOKApplicationVndAppleMpegurl{Data: r}, nil
	case "video/mp4":
		return &oas.GetEpisodeStreamFileOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported stream mime type: %s", mimeType)
	}
}

func (h *Handler) GetMovieSubtitleFile(ctx context.Context, params oas.GetMovieSubtitleFileParams) (oas.GetMovieSubtitleFileRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetMovieSubtitleFile(ctx, params.MovieId, params.SubtitleFile)
	if err != nil {
		return nil, providerError(err)
	}
	return movieSubtitleFileResponse(reader, mimeType)
}

func movieSubtitleFileResponse(r io.Reader, mimeType string) (oas.GetMovieSubtitleFileRes, error) {
	switch mimeType {
	case "application/ttml+xml":
		return &oas.GetMovieSubtitleFileOKApplicationTtmlXML{Data: r}, nil
	case "application/x-subrip":
		return &oas.GetMovieSubtitleFileOKApplicationXSubrip{Data: r}, nil
	case "text/plain":
		return &oas.GetMovieSubtitleFileOKTextPlain{Data: r}, nil
	case "text/vtt":
		return &oas.GetMovieSubtitleFileOKTextVtt{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported subtitle mime type: %s", mimeType)
	}
}

func (h *Handler) GetEpisodeSubtitleFile(ctx context.Context, params oas.GetEpisodeSubtitleFileParams) (oas.GetEpisodeSubtitleFileRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetEpisodeSubtitleFile(ctx, params.SeriesId, params.SeasonId, params.EpisodeId, params.SubtitleFile)
	if err != nil {
		return nil, providerError(err)
	}
	return episodeSubtitleFileResponse(reader, mimeType)
}

func episodeSubtitleFileResponse(r io.Reader, mimeType string) (oas.GetEpisodeSubtitleFileRes, error) {
	switch mimeType {
	case "application/ttml+xml":
		return &oas.GetEpisodeSubtitleFileOKApplicationTtmlXML{Data: r}, nil
	case "application/x-subrip":
		return &oas.GetEpisodeSubtitleFileOKApplicationXSubrip{Data: r}, nil
	case "text/plain":
		return &oas.GetEpisodeSubtitleFileOKTextPlain{Data: r}, nil
	case "text/vtt":
		return &oas.GetEpisodeSubtitleFileOKTextVtt{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported subtitle mime type: %s", mimeType)
	}
}

// --- Image response helpers ---

func moviePosterResponse(r io.Reader, mimeType string) (oas.GetMoviePosterRes, error) {
	switch mimeType {
	case "image/png":
		return &oas.GetMoviePosterOKImagePNG{Data: r}, nil
	case "image/jpeg":
		return &oas.GetMoviePosterOKImageJpeg{Data: r}, nil
	case "image/webp":
		return &oas.GetMoviePosterOKImageWEBP{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported image mime type: %s", mimeType)
	}
}

func movieBackdropResponse(r io.Reader, mimeType string) (oas.GetMovieBackdropRes, error) {
	switch mimeType {
	case "image/png":
		return &oas.GetMovieBackdropOKImagePNG{Data: r}, nil
	case "image/jpeg":
		return &oas.GetMovieBackdropOKImageJpeg{Data: r}, nil
	case "image/webp":
		return &oas.GetMovieBackdropOKImageWEBP{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported image mime type: %s", mimeType)
	}
}

func seriesPosterResponse(r io.Reader, mimeType string) (oas.GetSeriesPosterRes, error) {
	switch mimeType {
	case "image/png":
		return &oas.GetSeriesPosterOKImagePNG{Data: r}, nil
	case "image/jpeg":
		return &oas.GetSeriesPosterOKImageJpeg{Data: r}, nil
	case "image/webp":
		return &oas.GetSeriesPosterOKImageWEBP{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported image mime type: %s", mimeType)
	}
}

func seriesBackdropResponse(r io.Reader, mimeType string) (oas.GetSeriesBackdropRes, error) {
	switch mimeType {
	case "image/png":
		return &oas.GetSeriesBackdropOKImagePNG{Data: r}, nil
	case "image/jpeg":
		return &oas.GetSeriesBackdropOKImageJpeg{Data: r}, nil
	case "image/webp":
		return &oas.GetSeriesBackdropOKImageWEBP{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported image mime type: %s", mimeType)
	}
}

func seasonPosterResponse(r io.Reader, mimeType string) (oas.GetSeasonPosterRes, error) {
	switch mimeType {
	case "image/png":
		return &oas.GetSeasonPosterOKImagePNG{Data: r}, nil
	case "image/jpeg":
		return &oas.GetSeasonPosterOKImageJpeg{Data: r}, nil
	case "image/webp":
		return &oas.GetSeasonPosterOKImageWEBP{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported image mime type: %s", mimeType)
	}
}

func seasonBackdropResponse(r io.Reader, mimeType string) (oas.GetSeasonBackdropRes, error) {
	switch mimeType {
	case "image/png":
		return &oas.GetSeasonBackdropOKImagePNG{Data: r}, nil
	case "image/jpeg":
		return &oas.GetSeasonBackdropOKImageJpeg{Data: r}, nil
	case "image/webp":
		return &oas.GetSeasonBackdropOKImageWEBP{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported image mime type: %s", mimeType)
	}
}

func episodeThumbnailResponse(r io.Reader, mimeType string) (oas.GetEpisodeThumbnailRes, error) {
	switch mimeType {
	case "image/png":
		return &oas.GetEpisodeThumbnailOKImagePNG{Data: r}, nil
	case "image/jpeg":
		return &oas.GetEpisodeThumbnailOKImageJpeg{Data: r}, nil
	case "image/webp":
		return &oas.GetEpisodeThumbnailOKImageWEBP{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported image mime type: %s", mimeType)
	}
}
