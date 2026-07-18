package handler

import (
	"context"
	"fmt"
	"io"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/registry"
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

func (h *Handler) providerOrError(tag string) (provider.Provider, *oas.ErrorStatusCode) {
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

	var allItems []oas.SearchResultItem
	for _, p := range h.registry.All() {
		items, _, err := p.Search(ctx, params.Q, limit, offset)
		if err != nil {
			continue
		}
		allItems = append(allItems, items...)
	}

	return &oas.PageSearchResult{
		Items:  allItems,
		Total:  len(allItems),
		Limit:  limit,
		Offset: offset,
	}, nil
}

// --- GetServices ---

func (h *Handler) GetServices(ctx context.Context, params oas.GetServicesParams) (oas.GetServicesRes, error) {
	providers := h.registry.All()

	services := make([]oas.Service, 0, len(providers))

	for _, p := range providers {
		service, err := p.Service(ctx)
		if err != nil {
			return providerErrorMessage("upstreams unavailable"), nil
		}
		services = append(services, *service)
	}

	if params.Offset.Value > len(services) {
		return &oas.ErrorStatusCode{
			StatusCode: 400,
			Response:   oas.Error{Code: "BAD_REQUEST", Message: "offset was over total services amount"},
		}, nil
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
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	svc, err := p.Service(ctx)
	if err != nil {
		return providerError(err), nil
	}
	return svc, nil
}

// --- Movies ---

func (h *Handler) GetMovies(ctx context.Context, params oas.GetMoviesParams) (oas.GetMoviesRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	limit := params.Limit.Or(defaultLimit)
	offset := params.Offset.Or(0)
	items, total, err := p.GetMovies(ctx, limit, offset)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.PageMovie{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (h *Handler) GetMovieById(ctx context.Context, params oas.GetMovieByIdParams) (oas.GetMovieByIdRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	movie, err := p.GetMovieById(ctx, params.MovieId)
	if err != nil {
		return providerError(err), nil
	}
	return movie, nil
}

func (h *Handler) GetMovieStreams(ctx context.Context, params oas.GetMovieStreamsParams) (oas.GetMovieStreamsRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	items, total, err := p.GetMovieStreams(ctx, params.MovieId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.PageStream{Items: items, Total: total, Limit: params.Limit.Or(defaultLimit), Offset: params.Offset.Or(0)}, nil
}

func (h *Handler) GetMoviePoster(ctx context.Context, params oas.GetMoviePosterParams) (oas.GetMoviePosterRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, err := p.GetMoviePoster(ctx, params.MovieId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.GetMoviePosterOK{Data: reader}, nil
}

func (h *Handler) GetMovieBackdrop(ctx context.Context, params oas.GetMovieBackdropParams) (oas.GetMovieBackdropRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, err := p.GetMovieBackdrop(ctx, params.MovieId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.GetMovieBackdropOK{Data: reader}, nil
}

func (h *Handler) GetMovieSubtitles(ctx context.Context, params oas.GetMovieSubtitlesParams) (oas.GetMovieSubtitlesRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	items, total, err := p.GetMovieSubtitles(ctx, params.MovieId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.PageSubtitle{Items: items, Total: total, Limit: params.Limit.Or(defaultLimit), Offset: params.Offset.Or(0)}, nil
}

// --- Series ---

func (h *Handler) GetSeries(ctx context.Context, params oas.GetSeriesParams) (oas.GetSeriesRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	limit := params.Limit.Or(defaultLimit)
	offset := params.Offset.Or(0)
	items, total, err := p.GetSeries(ctx, limit, offset)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.PageSeries{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (h *Handler) GetSeriesById(ctx context.Context, params oas.GetSeriesByIdParams) (oas.GetSeriesByIdRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	series, err := p.GetSeriesByID(ctx, params.SeriesId)
	if err != nil {
		return providerError(err), nil
	}
	return series, nil
}

func (h *Handler) GetSeriesPoster(ctx context.Context, params oas.GetSeriesPosterParams) (oas.GetSeriesPosterRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, err := p.GetSeriesPoster(ctx, params.SeriesId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.GetSeriesPosterOK{Data: reader}, nil
}

func (h *Handler) GetSeriesBackdrop(ctx context.Context, params oas.GetSeriesBackdropParams) (oas.GetSeriesBackdropRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, err := p.GetSeriesBackdrop(ctx, params.SeriesId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.GetSeriesBackdropOK{Data: reader}, nil
}

// --- Seasons ---

func (h *Handler) GetSeasons(ctx context.Context, params oas.GetSeasonsParams) (oas.GetSeasonsRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	limit := params.Limit.Or(defaultLimit)
	offset := params.Offset.Or(0)
	items, total, err := p.GetSeasons(ctx, params.SeriesId, limit, offset)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.PageSeason{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (h *Handler) GetSeasonById(ctx context.Context, params oas.GetSeasonByIdParams) (oas.GetSeasonByIdRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	season, err := p.GetSeasonById(ctx, params.SeriesId, params.SeasonId)
	if err != nil {
		return providerError(err), nil
	}
	return season, nil
}

func (h *Handler) GetSeasonPoster(ctx context.Context, params oas.GetSeasonPosterParams) (oas.GetSeasonPosterRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, err := p.GetSeasonPoster(ctx, params.SeriesId, params.SeasonId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.GetSeasonPosterOK{Data: reader}, nil
}

func (h *Handler) GetSeasonBackdrop(ctx context.Context, params oas.GetSeasonBackdropParams) (oas.GetSeasonBackdropRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, err := p.GetSeasonBackdrop(ctx, params.SeriesId, params.SeasonId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.GetSeasonBackdropOK{Data: reader}, nil
}

// --- Episodes ---

func (h *Handler) GetEpisodes(ctx context.Context, params oas.GetEpisodesParams) (oas.GetEpisodesRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	limit := params.Limit.Or(defaultLimit)
	offset := params.Offset.Or(0)
	items, total, err := p.GetEpisodes(ctx, params.SeriesId, params.SeasonId, limit, offset)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.PageEpisode{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (h *Handler) GetEpisodeById(ctx context.Context, params oas.GetEpisodeByIdParams) (oas.GetEpisodeByIdRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	episode, err := p.GetEpisodeById(ctx, params.SeriesId, params.SeasonId, params.EpisodeId)
	if err != nil {
		return providerError(err), nil
	}
	return episode, nil
}

func (h *Handler) GetEpisodeStreams(ctx context.Context, params oas.GetEpisodeStreamsParams) (oas.GetEpisodeStreamsRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	items, total, err := p.GetEpisodeStreams(ctx, params.SeriesId, params.SeasonId, params.EpisodeId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.PageStream{Items: items, Total: total, Limit: params.Limit.Or(defaultLimit), Offset: params.Offset.Or(0)}, nil
}

func (h *Handler) GetEpisodeSubtitles(ctx context.Context, params oas.GetEpisodeSubtitlesParams) (oas.GetEpisodeSubtitlesRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	items, total, err := p.GetEpisodeSubtitles(ctx, params.SeriesId, params.SeasonId, params.EpisodeId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.PageSubtitle{Items: items, Total: total, Limit: params.Limit.Or(defaultLimit), Offset: params.Offset.Or(0)}, nil
}

func (h *Handler) GetEpisodeThumbnail(ctx context.Context, params oas.GetEpisodeThumbnailParams) (oas.GetEpisodeThumbnailRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, err := p.GetEpisodeThumbnail(ctx, params.SeriesId, params.SeasonId, params.EpisodeId)
	if err != nil {
		return providerError(err), nil
	}
	return &oas.GetEpisodeThumbnailOK{Data: reader}, nil
}

// --- Stream file methods ---

func (h *Handler) GetMovieStreamFile(ctx context.Context, params oas.GetMovieStreamFileParams) (oas.GetMovieStreamFileRes, error) {
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, mimeType, err := p.GetMovieStreamFile(ctx, params.MovieId, params.StreamFile)
	if err != nil {
		return providerError(err), nil
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
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, mimeType, err := p.GetEpisodeStreamFile(ctx, params.SeriesId, params.SeasonId, params.EpisodeId, params.StreamFile)
	if err != nil {
		return providerError(err), nil
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
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, mimeType, err := p.GetMovieSubtitleFile(ctx, params.MovieId, params.SubtitleFile)
	if err != nil {
		return providerError(err), nil
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
	p, errResp := h.providerOrError(params.ServiceTag)
	if errResp != nil {
		return errResp, nil
	}
	reader, mimeType, err := p.GetEpisodeSubtitleFile(ctx, params.SeriesId, params.SeasonId, params.EpisodeId, params.SubtitleFile)
	if err != nil {
		return providerError(err), nil
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
