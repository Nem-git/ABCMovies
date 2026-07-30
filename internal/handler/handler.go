package handler

import (
	"context"
	"fmt"
	"io"
	"path"
	"strconv"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/proxy"
	"github.com/nem-git/abcmovies/internal/registry"
	"github.com/nem-git/abcmovies/internal/search"
	"github.com/nem-git/abcmovies/internal/stream"
)

var _ oas.Handler = (*Handler)(nil)

const defaultLimit = 20

type Handler struct {
	oas.UnimplementedHandler
	registry *registry.Registry
	proxy    *proxy.Proxy
}

func New(r *registry.Registry, opts ...*proxy.Proxy) *Handler {
	var p *proxy.Proxy
	if len(opts) > 0 {
		p = opts[0]
	}
	return &Handler{registry: r, proxy: p}
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

func proxyError(err error) *oas.ErrorStatusCode {
	return providerErrorMessage(fmt.Sprintf("proxy: %v", err))
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
	format := string(params.Format)
	locator, err := p.GetMovieStreamLocator(ctx, params.MovieId, params.File)
	if err != nil {
		return nil, providerError(err)
	}

	// Direct serve: provider returns Data with no URL
	if locator.Data != nil && locator.URL == "" {
		return movieStreamFileResponse(locator)
	}

	// Proxy serve: provider returns upstream URL
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	proxyBaseURL := path.Clean(fmt.Sprintf("/services/%s/movies/%s/streams/%s", params.ServiceTag, params.MovieId, format))
	reader, contentType, err := h.proxy.ServeManifest(ctx, params.ServiceTag, contentKey, format, params.File, *locator, proxyBaseURL)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieStreamFileResponseByType(reader, contentType)
}

func (h *Handler) GetMovieStreamSegment(ctx context.Context, params oas.GetMovieStreamSegmentParams) (oas.GetMovieStreamSegmentRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	reader, contentType, err := h.proxy.ServeSegment(ctx, params.ServiceTag, contentKey, format, params.Rendition, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetEpisodeStreamFile(ctx context.Context, params oas.GetEpisodeStreamFileParams) (oas.GetEpisodeStreamFileRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	format := string(params.Format)
	locator, err := p.GetEpisodeStreamLocator(ctx, params.SeriesId, params.SeasonId, params.EpisodeId, params.File)
	if err != nil {
		return nil, providerError(err)
	}

	// Direct serve: provider returns Data with no URL
	if locator.Data != nil && locator.URL == "" {
		return episodeStreamFileResponse(locator)
	}

	// Proxy serve: provider returns upstream URL
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	proxyBaseURL := path.Clean(fmt.Sprintf("/services/%s/series/%s/seasons/%s/episodes/%s/streams/%s", params.ServiceTag, params.SeriesId, params.SeasonId, params.EpisodeId, format))
	reader, contentType, err := h.proxy.ServeManifest(ctx, params.ServiceTag, contentKey, format, params.File, *locator, proxyBaseURL)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeStreamFileResponseByType(reader, contentType)
}

func (h *Handler) GetEpisodeStreamSegment(ctx context.Context, params oas.GetEpisodeStreamSegmentParams) (oas.GetEpisodeStreamSegmentRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, contentType, err := h.proxy.ServeSegment(ctx, params.ServiceTag, contentKey, format, params.Rendition, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeSegmentResponseByType(reader, contentType)
}

// --- Movie HLS variant/rendition/DASH endpoints ---

func (h *Handler) GetMovieHLSVariant(ctx context.Context, params oas.GetMovieHLSVariantParams) (oas.GetMovieHLSVariantRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	reader, _, err := h.proxy.ServeHLSSubPlaylist(ctx, params.ServiceTag, contentKey, "hls", "variants", strconv.Itoa(params.VariantIndex))
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetMovieHLSVariantOK{Data: reader}, nil
}

func (h *Handler) GetMovieHLSVariantSegment(ctx context.Context, params oas.GetMovieHLSVariantSegmentParams) (oas.GetMovieHLSVariantSegmentRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	reader, contentType, err := h.proxy.ServeHLSSegment(ctx, params.ServiceTag, contentKey, "hls", "variants", strconv.Itoa(params.VariantIndex), params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSVariantSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetMovieHLSRendition(ctx context.Context, params oas.GetMovieHLSRenditionParams) (oas.GetMovieHLSRenditionRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	renditionID := params.GroupId + "/" + params.RenditionName
	reader, _, err := h.proxy.ServeHLSSubPlaylist(ctx, params.ServiceTag, contentKey, "hls", "renditions", renditionID)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetMovieHLSRenditionOK{Data: reader}, nil
}

func (h *Handler) GetMovieHLSRenditionSegment(ctx context.Context, params oas.GetMovieHLSRenditionSegmentParams) (oas.GetMovieHLSRenditionSegmentRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	renditionID := params.GroupId + "/" + params.RenditionName
	reader, contentType, err := h.proxy.ServeHLSSegment(ctx, params.ServiceTag, contentKey, "hls", "renditions", renditionID, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSRenditionSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetMovieDASHSegment(ctx context.Context, params oas.GetMovieDASHSegmentParams) (oas.GetMovieDASHSegmentRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	reader, contentType, err := h.proxy.ServeDASHSegment(ctx, params.ServiceTag, contentKey, format, params.Period, params.AdaptationSet, params.Representation, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieDashSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetMovieDASHInit(ctx context.Context, params oas.GetMovieDASHInitParams) (oas.GetMovieDASHInitRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	reader, contentType, err := h.proxy.ServeDASHInit(ctx, params.ServiceTag, contentKey, format, params.Period, params.AdaptationSet, params.Representation)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieDashInitResponseByType(reader, contentType)
}

// --- Episode HLS variant/rendition/DASH endpoints ---

func (h *Handler) GetEpisodeHLSVariant(ctx context.Context, params oas.GetEpisodeHLSVariantParams) (oas.GetEpisodeHLSVariantRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, _, err := h.proxy.ServeHLSSubPlaylist(ctx, params.ServiceTag, contentKey, "hls", "variants", strconv.Itoa(params.VariantIndex))
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetEpisodeHLSVariantOK{Data: reader}, nil
}

func (h *Handler) GetEpisodeHLSVariantSegment(ctx context.Context, params oas.GetEpisodeHLSVariantSegmentParams) (oas.GetEpisodeHLSVariantSegmentRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, contentType, err := h.proxy.ServeHLSSegment(ctx, params.ServiceTag, contentKey, "hls", "variants", strconv.Itoa(params.VariantIndex), params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSVariantSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetEpisodeHLSRendition(ctx context.Context, params oas.GetEpisodeHLSRenditionParams) (oas.GetEpisodeHLSRenditionRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	renditionID := params.GroupId + "/" + params.RenditionName
	reader, _, err := h.proxy.ServeHLSSubPlaylist(ctx, params.ServiceTag, contentKey, "hls", "renditions", renditionID)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetEpisodeHLSRenditionOK{Data: reader}, nil
}

func (h *Handler) GetEpisodeHLSRenditionSegment(ctx context.Context, params oas.GetEpisodeHLSRenditionSegmentParams) (oas.GetEpisodeHLSRenditionSegmentRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	renditionID := params.GroupId + "/" + params.RenditionName
	reader, contentType, err := h.proxy.ServeHLSSegment(ctx, params.ServiceTag, contentKey, "hls", "renditions", renditionID, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSRenditionSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetEpisodeDASHSegment(ctx context.Context, params oas.GetEpisodeDASHSegmentParams) (oas.GetEpisodeDASHSegmentRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, contentType, err := h.proxy.ServeDASHSegment(ctx, params.ServiceTag, contentKey, format, params.Period, params.AdaptationSet, params.Representation, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeDashSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetEpisodeDASHInit(ctx context.Context, params oas.GetEpisodeDASHInitParams) (oas.GetEpisodeDASHInitRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, contentType, err := h.proxy.ServeDASHInit(ctx, params.ServiceTag, contentKey, format, params.Period, params.AdaptationSet, params.Representation)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeDashInitResponseByType(reader, contentType)
}

// --- Stream response helpers ---

func movieStreamFileResponseByType(r io.Reader, contentType string) (oas.GetMovieStreamFileRes, error) {
	switch contentType {
	case "application/dash+xml":
		return &oas.GetMovieStreamFileOKApplicationDashXML{Data: r}, nil
	case "application/vnd.apple.mpegurl":
		return &oas.GetMovieStreamFileOKApplicationVndAppleMpegurl{Data: r}, nil
	case "video/mp4":
		return &oas.GetMovieStreamFileOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported stream content type: %s", contentType)
	}
}

func episodeStreamFileResponseByType(r io.Reader, contentType string) (oas.GetEpisodeStreamFileRes, error) {
	switch contentType {
	case "application/dash+xml":
		return &oas.GetEpisodeStreamFileOKApplicationDashXML{Data: r}, nil
	case "application/vnd.apple.mpegurl":
		return &oas.GetEpisodeStreamFileOKApplicationVndAppleMpegurl{Data: r}, nil
	case "video/mp4":
		return &oas.GetEpisodeStreamFileOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported stream content type: %s", contentType)
	}
}

func movieSegmentResponseByType(r io.Reader, contentType string) (oas.GetMovieStreamSegmentRes, error) {
	switch contentType {
	case "video/mp2t":
		return &oas.GetMovieStreamSegmentOKVideoMp2t{Data: r}, nil
	case "video/mp4":
		return &oas.GetMovieStreamSegmentOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported segment content type: %s", contentType)
	}
}

func episodeSegmentResponseByType(r io.Reader, contentType string) (oas.GetEpisodeStreamSegmentRes, error) {
	switch contentType {
	case "video/mp2t":
		return &oas.GetEpisodeStreamSegmentOKVideoMp2t{Data: r}, nil
	case "video/mp4":
		return &oas.GetEpisodeStreamSegmentOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported segment content type: %s", contentType)
	}
}

func movieDashSegmentResponseByType(r io.Reader, contentType string) (oas.GetMovieDASHSegmentRes, error) {
	switch contentType {
	case "video/mp4":
		return &oas.GetMovieDASHSegmentOK{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported DASH segment content type: %s", contentType)
	}
}

func movieHLSVariantSegmentResponseByType(r io.Reader, contentType string) (oas.GetMovieHLSVariantSegmentRes, error) {
	switch contentType {
	case "video/mp2t":
		return &oas.GetMovieHLSVariantSegmentOKVideoMp2t{Data: r}, nil
	case "video/mp4":
		return &oas.GetMovieHLSVariantSegmentOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported segment content type: %s", contentType)
	}
}

func movieHLSRenditionSegmentResponseByType(r io.Reader, contentType string) (oas.GetMovieHLSRenditionSegmentRes, error) {
	switch contentType {
	case "video/mp2t":
		return &oas.GetMovieHLSRenditionSegmentOKVideoMp2t{Data: r}, nil
	case "video/mp4":
		return &oas.GetMovieHLSRenditionSegmentOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported segment content type: %s", contentType)
	}
}

func movieDashInitResponseByType(r io.Reader, contentType string) (oas.GetMovieDASHInitRes, error) {
	switch contentType {
	case "video/mp4":
		return &oas.GetMovieDASHInitOK{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported DASH init content type: %s", contentType)
	}
}

func episodeDashSegmentResponseByType(r io.Reader, contentType string) (oas.GetEpisodeDASHSegmentRes, error) {
	switch contentType {
	case "video/mp4":
		return &oas.GetEpisodeDASHSegmentOK{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported DASH segment content type: %s", contentType)
	}
}

func episodeHLSVariantSegmentResponseByType(r io.Reader, contentType string) (oas.GetEpisodeHLSVariantSegmentRes, error) {
	switch contentType {
	case "video/mp2t":
		return &oas.GetEpisodeHLSVariantSegmentOKVideoMp2t{Data: r}, nil
	case "video/mp4":
		return &oas.GetEpisodeHLSVariantSegmentOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported segment content type: %s", contentType)
	}
}

func episodeHLSRenditionSegmentResponseByType(r io.Reader, contentType string) (oas.GetEpisodeHLSRenditionSegmentRes, error) {
	switch contentType {
	case "video/mp2t":
		return &oas.GetEpisodeHLSRenditionSegmentOKVideoMp2t{Data: r}, nil
	case "video/mp4":
		return &oas.GetEpisodeHLSRenditionSegmentOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported segment content type: %s", contentType)
	}
}

func episodeDashInitResponseByType(r io.Reader, contentType string) (oas.GetEpisodeDASHInitRes, error) {
	switch contentType {
	case "video/mp4":
		return &oas.GetEpisodeDASHInitOK{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported DASH init content type: %s", contentType)
	}
}

func movieStreamFileResponse(locator *stream.Locator) (oas.GetMovieStreamFileRes, error) {
	r := locator.Data
	if r == nil {
		r = io.NopCloser(nil)
	}
	switch locator.EncodingFormat {
	case "application/dash+xml":
		return &oas.GetMovieStreamFileOKApplicationDashXML{Data: r}, nil
	case "application/vnd.apple.mpegurl":
		return &oas.GetMovieStreamFileOKApplicationVndAppleMpegurl{Data: r}, nil
	case "video/mp4":
		return &oas.GetMovieStreamFileOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported stream mime type: %s", locator.EncodingFormat)
	}
}

func episodeStreamFileResponse(locator *stream.Locator) (oas.GetEpisodeStreamFileRes, error) {
	r := locator.Data
	if r == nil {
		r = io.NopCloser(nil)
	}
	switch locator.EncodingFormat {
	case "application/dash+xml":
		return &oas.GetEpisodeStreamFileOKApplicationDashXML{Data: r}, nil
	case "application/vnd.apple.mpegurl":
		return &oas.GetEpisodeStreamFileOKApplicationVndAppleMpegurl{Data: r}, nil
	case "video/mp4":
		return &oas.GetEpisodeStreamFileOKVideoMP4{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported stream mime type: %s", locator.EncodingFormat)
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

// ── Movie HLS Resource Methods ─────────────────────────────────────────────

func (h *Handler) GetMovieVariantKey(ctx context.Context, params oas.GetMovieVariantKeyParams) (oas.GetMovieVariantKeyRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	resourceID := strconv.Itoa(params.VariantIndex) + "/" + params.File
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "key", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetMovieVariantKeyOK{Data: reader}, nil
}

func (h *Handler) GetMovieVariantPartial(ctx context.Context, params oas.GetMovieVariantPartialParams) (oas.GetMovieVariantPartialRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	resourceID := strconv.Itoa(params.VariantIndex) + "/" + params.File
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "partial", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieVariantPartialResponse(reader, ct)
}

func (h *Handler) GetMovieVariantPreloadHint(ctx context.Context, params oas.GetMovieVariantPreloadHintParams) (oas.GetMovieVariantPreloadHintRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	resourceID := strconv.Itoa(params.VariantIndex) + "/" + params.File
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "preload-hint", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieVariantPreloadHintResponse(reader, ct)
}

func (h *Handler) GetMovieRenditionKey(ctx context.Context, params oas.GetMovieRenditionKeyParams) (oas.GetMovieRenditionKeyRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	resourceID := params.GroupId + "/" + params.RenditionName + "/" + params.File
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "key", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetMovieRenditionKeyOK{Data: reader}, nil
}

func (h *Handler) GetMovieRenditionPartial(ctx context.Context, params oas.GetMovieRenditionPartialParams) (oas.GetMovieRenditionPartialRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	resourceID := params.GroupId + "/" + params.RenditionName + "/" + params.File
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "partial", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieRenditionPartialResponse(reader, ct)
}

func (h *Handler) GetMovieRenditionPreloadHint(ctx context.Context, params oas.GetMovieRenditionPreloadHintParams) (oas.GetMovieRenditionPreloadHintRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	resourceID := params.GroupId + "/" + params.RenditionName + "/" + params.File
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "preload-hint", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieRenditionPreloadHintResponse(reader, ct)
}

func (h *Handler) GetMovieHLSSessionKey(ctx context.Context, params oas.GetMovieHLSSessionKeyParams) (oas.GetMovieHLSSessionKeyRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "session-key", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetMovieHLSSessionKeyOK{Data: reader}, nil
}

func (h *Handler) GetMovieHLSSessionData(ctx context.Context, params oas.GetMovieHLSSessionDataParams) (oas.GetMovieHLSSessionDataRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "session-data", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSSessionDataResponse(reader, ct)
}

func (h *Handler) GetMovieHLSSteering(ctx context.Context, params oas.GetMovieHLSSteeringParams) (oas.GetMovieHLSSteeringRes, error) {
	contentKey := proxy.BuildContentKey("movie", params.MovieId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "steering", "")
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSSteeringResponse(reader, ct)
}

// ── Episode HLS Resource Methods ──────────────────────────────────────────

func (h *Handler) GetEpisodeVariantKey(ctx context.Context, params oas.GetEpisodeVariantKeyParams) (oas.GetEpisodeVariantKeyRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	resourceID := strconv.Itoa(params.VariantIndex) + "/" + params.File
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "key", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetEpisodeVariantKeyOK{Data: reader}, nil
}

func (h *Handler) GetEpisodeVariantPartial(ctx context.Context, params oas.GetEpisodeVariantPartialParams) (oas.GetEpisodeVariantPartialRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	resourceID := strconv.Itoa(params.VariantIndex) + "/" + params.File
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "partial", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeVariantPartialResponse(reader, ct)
}

func (h *Handler) GetEpisodeVariantPreloadHint(ctx context.Context, params oas.GetEpisodeVariantPreloadHintParams) (oas.GetEpisodeVariantPreloadHintRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	resourceID := strconv.Itoa(params.VariantIndex) + "/" + params.File
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "preload-hint", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeVariantPreloadHintResponse(reader, ct)
}

func (h *Handler) GetEpisodeRenditionKey(ctx context.Context, params oas.GetEpisodeRenditionKeyParams) (oas.GetEpisodeRenditionKeyRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	resourceID := params.GroupId + "/" + params.RenditionName + "/" + params.File
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "key", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetEpisodeRenditionKeyOK{Data: reader}, nil
}

func (h *Handler) GetEpisodeRenditionPartial(ctx context.Context, params oas.GetEpisodeRenditionPartialParams) (oas.GetEpisodeRenditionPartialRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	resourceID := params.GroupId + "/" + params.RenditionName + "/" + params.File
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "partial", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeRenditionPartialResponse(reader, ct)
}

func (h *Handler) GetEpisodeRenditionPreloadHint(ctx context.Context, params oas.GetEpisodeRenditionPreloadHintParams) (oas.GetEpisodeRenditionPreloadHintRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	resourceID := params.GroupId + "/" + params.RenditionName + "/" + params.File
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "preload-hint", resourceID)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeRenditionPreloadHintResponse(reader, ct)
}

func (h *Handler) GetEpisodeHLSSessionKey(ctx context.Context, params oas.GetEpisodeHLSSessionKeyParams) (oas.GetEpisodeHLSSessionKeyRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "session-key", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetEpisodeHLSSessionKeyOK{Data: reader}, nil
}

func (h *Handler) GetEpisodeHLSSessionData(ctx context.Context, params oas.GetEpisodeHLSSessionDataParams) (oas.GetEpisodeHLSSessionDataRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "session-data", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSSessionDataResponse(reader, ct)
}

func (h *Handler) GetEpisodeHLSSteering(ctx context.Context, params oas.GetEpisodeHLSSteeringParams) (oas.GetEpisodeHLSSteeringRes, error) {
	contentKey := proxy.BuildContentKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "steering", "")
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSSteeringResponse(reader, ct)
}

// ── HLS Resource Response Helpers ─────────────────────────────────────────

func movieVariantPartialResponse(r io.Reader, ct string) (oas.GetMovieVariantPartialRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetMovieVariantPartialOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetMovieVariantPartialOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetMovieVariantPartialOKVideoMp2t{Data: r}, nil
	}
}

func movieVariantPreloadHintResponse(r io.Reader, ct string) (oas.GetMovieVariantPreloadHintRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetMovieVariantPreloadHintOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetMovieVariantPreloadHintOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetMovieVariantPreloadHintOKVideoMp2t{Data: r}, nil
	}
}

func movieRenditionPartialResponse(r io.Reader, ct string) (oas.GetMovieRenditionPartialRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetMovieRenditionPartialOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetMovieRenditionPartialOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetMovieRenditionPartialOKVideoMp2t{Data: r}, nil
	}
}

func movieRenditionPreloadHintResponse(r io.Reader, ct string) (oas.GetMovieRenditionPreloadHintRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetMovieRenditionPreloadHintOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetMovieRenditionPreloadHintOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetMovieRenditionPreloadHintOKVideoMp2t{Data: r}, nil
	}
}

func movieHLSSessionDataResponse(r io.Reader, ct string) (oas.GetMovieHLSSessionDataRes, error) {
	switch ct {
	case "application/json":
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		s := oas.GetMovieHLSSessionDataOKApplicationJSON(string(data))
		return &s, nil
	default:
		return &oas.GetMovieHLSSessionDataOKTextPlain{Data: r}, nil
	}
}

func movieHLSSteeringResponse(r io.Reader, ct string) (oas.GetMovieHLSSteeringRes, error) {
	switch ct {
	case "application/json":
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		s := oas.GetMovieHLSSteeringOKApplicationJSON(string(data))
		return &s, nil
	default:
		return &oas.GetMovieHLSSteeringOKApplicationVndAppleMpegurl{Data: r}, nil
	}
}

func episodeVariantPartialResponse(r io.Reader, ct string) (oas.GetEpisodeVariantPartialRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetEpisodeVariantPartialOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetEpisodeVariantPartialOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetEpisodeVariantPartialOKVideoMp2t{Data: r}, nil
	}
}

func episodeVariantPreloadHintResponse(r io.Reader, ct string) (oas.GetEpisodeVariantPreloadHintRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetEpisodeVariantPreloadHintOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetEpisodeVariantPreloadHintOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetEpisodeVariantPreloadHintOKVideoMp2t{Data: r}, nil
	}
}

func episodeRenditionPartialResponse(r io.Reader, ct string) (oas.GetEpisodeRenditionPartialRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetEpisodeRenditionPartialOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetEpisodeRenditionPartialOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetEpisodeRenditionPartialOKVideoMp2t{Data: r}, nil
	}
}

func episodeRenditionPreloadHintResponse(r io.Reader, ct string) (oas.GetEpisodeRenditionPreloadHintRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetEpisodeRenditionPreloadHintOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetEpisodeRenditionPreloadHintOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetEpisodeRenditionPreloadHintOKVideoMp2t{Data: r}, nil
	}
}

func episodeHLSSessionDataResponse(r io.Reader, ct string) (oas.GetEpisodeHLSSessionDataRes, error) {
	switch ct {
	case "application/json":
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		s := oas.GetEpisodeHLSSessionDataOKApplicationJSON(string(data))
		return &s, nil
	default:
		return &oas.GetEpisodeHLSSessionDataOKTextPlain{Data: r}, nil
	}
}

func episodeHLSSteeringResponse(r io.Reader, ct string) (oas.GetEpisodeHLSSteeringRes, error) {
	switch ct {
	case "application/json":
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		s := oas.GetEpisodeHLSSteeringOKApplicationJSON(string(data))
		return &s, nil
	default:
		return &oas.GetEpisodeHLSSteeringOKApplicationVndAppleMpegurl{Data: r}, nil
	}
}
