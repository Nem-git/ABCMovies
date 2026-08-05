package handler

import (
	"context"
	"fmt"
	"io"
	"path"

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
	baseURL  string
	apiPrefix string
}

func New(r *registry.Registry, baseURL string, apiPrefix string, opts ...*proxy.Proxy) *Handler {
	var p *proxy.Proxy
	if len(opts) > 0 {
		p = opts[0]
	}
	return &Handler{registry: r, proxy: p, baseURL: baseURL, apiPrefix: apiPrefix}
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
	for i := range items {
		h.absolutizeSearchItem(ctx, &items[i])
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
	for i := range services {
		h.absolutizeService(ctx, &services[i])
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
	h.absolutizeService(ctx, svc)
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
	for i := range items {
		h.absolutizeMovie(ctx, &items[i])
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
	h.absolutizeMovie(ctx, movie)
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
	for i := range items {
		h.absolutizeSubtitle(ctx, &items[i])
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
	for i := range items {
		h.absolutizeSeries(ctx, &items[i])
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
	h.absolutizeSeries(ctx, series)
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
	for i := range items {
		h.absolutizeSeason(ctx, &items[i])
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
	h.absolutizeSeason(ctx, season)
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
	for i := range items {
		h.absolutizeEpisode(ctx, &items[i])
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
	h.absolutizeEpisode(ctx, episode)
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
	for i := range items {
		h.absolutizeSubtitle(ctx, &items[i])
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
	file := manifestFileName(format)
	locator, err := p.GetMovieStreamLocator(ctx, params.MovieId, file)
	if err != nil {
		return nil, providerError(err)
	}

	// Direct serve: provider returns Data with no URL
	if locator.Data != nil && locator.URL == "" {
		return movieStreamFileResponse(locator)
	}

	// Proxy serve: provider returns upstream URL
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	proxyBaseURL := path.Clean(h.apiPrefix + fmt.Sprintf("/services/%s/movies/%s/streams/%s", params.ServiceTag, params.MovieId, format))
	reader, contentType, err := h.proxy.ServeManifest(ctx, params.ServiceTag, contentKey, format, file, *locator, proxyBaseURL)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieStreamFileResponseByType(reader, contentType)
}

func (h *Handler) GetEpisodeStreamFile(ctx context.Context, params oas.GetEpisodeStreamFileParams) (oas.GetEpisodeStreamFileRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	format := string(params.Format)
	file := manifestFileName(format)
	locator, err := p.GetEpisodeStreamLocator(ctx, params.SeriesId, params.SeasonId, params.EpisodeId, file)
	if err != nil {
		return nil, providerError(err)
	}

	// Direct serve: provider returns Data with no URL
	if locator.Data != nil && locator.URL == "" {
		return episodeStreamFileResponse(locator)
	}
	// Proxy serve: provider returns upstream URL
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	proxyBaseURL := path.Clean(h.apiPrefix + fmt.Sprintf("/services/%s/series/%s/seasons/%s/episodes/%s/streams/%s", params.ServiceTag, params.SeriesId, params.SeasonId, params.EpisodeId, format))
	reader, contentType, err := h.proxy.ServeManifest(ctx, params.ServiceTag, contentKey, format, file, *locator, proxyBaseURL)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeStreamFileResponseByType(reader, contentType)
}

// --- Movie HLS variant/rendition/DASH endpoints ---

func (h *Handler) GetMovieHLSVariant(ctx context.Context, params oas.GetMovieHLSVariantParams) (oas.GetMovieHLSVariantRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, _, err := h.proxy.ServeHLSSubPlaylist(ctx, params.ServiceTag, contentKey, "hls", "variants", params.VariantId)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetMovieHLSVariantOK{Data: reader}, nil
}

func (h *Handler) GetMovieHLSVariantSegment(ctx context.Context, params oas.GetMovieHLSVariantSegmentParams) (oas.GetMovieHLSVariantSegmentRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, contentType, err := h.proxy.ServeHLSSegment(ctx, params.ServiceTag, contentKey, "hls", "variants", params.VariantId, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSVariantSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetMovieHLSRendition(ctx context.Context, params oas.GetMovieHLSRenditionParams) (oas.GetMovieHLSRenditionRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	renditionID := params.GroupId + "/" + params.RenditionName
	reader, _, err := h.proxy.ServeHLSSubPlaylist(ctx, params.ServiceTag, contentKey, "hls", "renditions", renditionID)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetMovieHLSRenditionOK{Data: reader}, nil
}

func (h *Handler) GetMovieHLSRenditionSegment(ctx context.Context, params oas.GetMovieHLSRenditionSegmentParams) (oas.GetMovieHLSRenditionSegmentRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	renditionID := params.GroupId + "/" + params.RenditionName
	reader, contentType, err := h.proxy.ServeHLSSegment(ctx, params.ServiceTag, contentKey, "hls", "renditions", renditionID, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSRenditionSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetMovieDASHSegment(ctx context.Context, params oas.GetMovieDASHSegmentParams) (oas.GetMovieDASHSegmentRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, contentType, err := h.proxy.ServeDASHSegment(ctx, params.ServiceTag, contentKey, format, params.Period, params.AdaptationSet, params.Representation, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieDashSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetMovieDASHInit(ctx context.Context, params oas.GetMovieDASHInitParams) (oas.GetMovieDASHInitRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, contentType, err := h.proxy.ServeDASHInit(ctx, params.ServiceTag, contentKey, format, params.Period, params.AdaptationSet, params.Representation)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieDashInitResponseByType(reader, contentType)
}

// --- Episode HLS variant/rendition/DASH endpoints ---

func (h *Handler) GetEpisodeHLSVariant(ctx context.Context, params oas.GetEpisodeHLSVariantParams) (oas.GetEpisodeHLSVariantRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, _, err := h.proxy.ServeHLSSubPlaylist(ctx, params.ServiceTag, contentKey, "hls", "variants", params.VariantId)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetEpisodeHLSVariantOK{Data: reader}, nil
}

func (h *Handler) GetEpisodeHLSVariantSegment(ctx context.Context, params oas.GetEpisodeHLSVariantSegmentParams) (oas.GetEpisodeHLSVariantSegmentRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, contentType, err := h.proxy.ServeHLSSegment(ctx, params.ServiceTag, contentKey, "hls", "variants", params.VariantId, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSVariantSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetEpisodeHLSRendition(ctx context.Context, params oas.GetEpisodeHLSRenditionParams) (oas.GetEpisodeHLSRenditionRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	renditionID := params.GroupId + "/" + params.RenditionName
	reader, _, err := h.proxy.ServeHLSSubPlaylist(ctx, params.ServiceTag, contentKey, "hls", "renditions", renditionID)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetEpisodeHLSRenditionOK{Data: reader}, nil
}

func (h *Handler) GetEpisodeHLSRenditionSegment(ctx context.Context, params oas.GetEpisodeHLSRenditionSegmentParams) (oas.GetEpisodeHLSRenditionSegmentRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	renditionID := params.GroupId + "/" + params.RenditionName
	reader, contentType, err := h.proxy.ServeHLSSegment(ctx, params.ServiceTag, contentKey, "hls", "renditions", renditionID, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSRenditionSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetEpisodeDASHSegment(ctx context.Context, params oas.GetEpisodeDASHSegmentParams) (oas.GetEpisodeDASHSegmentRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, contentType, err := h.proxy.ServeDASHSegment(ctx, params.ServiceTag, contentKey, format, params.Period, params.AdaptationSet, params.Representation, params.Segment)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeDashSegmentResponseByType(reader, contentType)
}

func (h *Handler) GetEpisodeDASHInit(ctx context.Context, params oas.GetEpisodeDASHInitParams) (oas.GetEpisodeDASHInitRes, error) {
	format := string(params.Format)
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, contentType, err := h.proxy.ServeDASHInit(ctx, params.ServiceTag, contentKey, format, params.Period, params.AdaptationSet, params.Representation)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeDashInitResponseByType(reader, contentType)
}

// --- Stream response helpers ---

// manifestFileName returns the canonical upstream manifest filename for a
// stream format. Used where the API route carries no explicit file name
// (episode stream files are served at .../streams/{format}).
func manifestFileName(format string) string {
	switch format {
	case "hls":
		return "master.m3u8"
	case "dash":
		return "manifest.mpd"
	default:
		return "video.mp4"
	}
}

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

func (h *Handler) GetMovieSubtitle(ctx context.Context, params oas.GetMovieSubtitleParams) (oas.GetMovieSubtitleRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetMovieSubtitleFile(ctx, params.MovieId, params.SubtitleId)
	if err != nil {
		return nil, providerError(err)
	}
	return movieSubtitleResponse(reader, mimeType)
}

func movieSubtitleResponse(r io.Reader, mimeType string) (oas.GetMovieSubtitleRes, error) {
	switch mimeType {
	case "application/ttml+xml":
		return &oas.GetMovieSubtitleOKApplicationTtmlXML{Data: r}, nil
	case "application/x-subrip":
		return &oas.GetMovieSubtitleOKApplicationXSubrip{Data: r}, nil
	case "text/plain":
		return &oas.GetMovieSubtitleOKTextPlain{Data: r}, nil
	case "text/vtt":
		return &oas.GetMovieSubtitleOKTextVtt{Data: r}, nil
	default:
		return nil, fmt.Errorf("unsupported subtitle mime type: %s", mimeType)
	}
}

func (h *Handler) GetEpisodeSubtitle(ctx context.Context, params oas.GetEpisodeSubtitleParams) (oas.GetEpisodeSubtitleRes, error) {
	p, err := h.providerOrError(params.ServiceTag)
	if err != nil {
		return nil, err
	}
	reader, mimeType, err := p.GetEpisodeSubtitleFile(ctx, params.SeriesId, params.SeasonId, params.EpisodeId, params.SubtitleId)
	if err != nil {
		return nil, providerError(err)
	}
	return episodeSubtitleResponse(reader, mimeType)
}

func episodeSubtitleResponse(r io.Reader, mimeType string) (oas.GetEpisodeSubtitleRes, error) {
	switch mimeType {
	case "application/ttml+xml":
		return &oas.GetEpisodeSubtitleOKApplicationTtmlXML{Data: r}, nil
	case "application/x-subrip":
		return &oas.GetEpisodeSubtitleOKApplicationXSubrip{Data: r}, nil
	case "text/plain":
		return &oas.GetEpisodeSubtitleOKTextPlain{Data: r}, nil
	case "text/vtt":
		return &oas.GetEpisodeSubtitleOKTextVtt{Data: r}, nil
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

func (h *Handler) GetMovieHLSKey(ctx context.Context, params oas.GetMovieHLSKeyParams) (oas.GetMovieHLSKeyRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "key", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetMovieHLSKeyOK{Data: reader}, nil
}

func (h *Handler) GetMovieHLSPartial(ctx context.Context, params oas.GetMovieHLSPartialParams) (oas.GetMovieHLSPartialRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "partial", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSPartialResponse(reader, ct)
}

func (h *Handler) GetMovieHLSPreloadHint(ctx context.Context, params oas.GetMovieHLSPreloadHintParams) (oas.GetMovieHLSPreloadHintRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "preload-hint", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSPreloadHintResponse(reader, ct)
}

func (h *Handler) GetMovieHLSSessionKey(ctx context.Context, params oas.GetMovieHLSSessionKeyParams) (oas.GetMovieHLSSessionKeyRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "session-key", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetMovieHLSSessionKeyOK{Data: reader}, nil
}

func (h *Handler) GetMovieHLSSessionData(ctx context.Context, params oas.GetMovieHLSSessionDataParams) (oas.GetMovieHLSSessionDataRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "session-data", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSSessionDataResponse(reader, ct)
}

func (h *Handler) GetMovieHLSSteering(ctx context.Context, params oas.GetMovieHLSSteeringParams) (oas.GetMovieHLSSteeringRes, error) {
	contentKey := proxy.BuildStateKey("movie", params.MovieId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "steering", "")
	if err != nil {
		return nil, proxyError(err)
	}
	return movieHLSSteeringResponse(reader, ct)
}

// ── Episode HLS Resource Methods ──────────────────────────────────────────

func (h *Handler) GetEpisodeHLSKey(ctx context.Context, params oas.GetEpisodeHLSKeyParams) (oas.GetEpisodeHLSKeyRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "key", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetEpisodeHLSKeyOK{Data: reader}, nil
}

func (h *Handler) GetEpisodeHLSPartial(ctx context.Context, params oas.GetEpisodeHLSPartialParams) (oas.GetEpisodeHLSPartialRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "partial", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSPartialResponse(reader, ct)
}

func (h *Handler) GetEpisodeHLSPreloadHint(ctx context.Context, params oas.GetEpisodeHLSPreloadHintParams) (oas.GetEpisodeHLSPreloadHintRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "preload-hint", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSPreloadHintResponse(reader, ct)
}

func (h *Handler) GetEpisodeHLSSessionKey(ctx context.Context, params oas.GetEpisodeHLSSessionKeyParams) (oas.GetEpisodeHLSSessionKeyRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, _, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "session-key", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return &oas.GetEpisodeHLSSessionKeyOK{Data: reader}, nil
}

func (h *Handler) GetEpisodeHLSSessionData(ctx context.Context, params oas.GetEpisodeHLSSessionDataParams) (oas.GetEpisodeHLSSessionDataRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "session-data", params.File)
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSSessionDataResponse(reader, ct)
}

func (h *Handler) GetEpisodeHLSSteering(ctx context.Context, params oas.GetEpisodeHLSSteeringParams) (oas.GetEpisodeHLSSteeringRes, error) {
	contentKey := proxy.BuildStateKey("episode", params.SeriesId, params.SeasonId, params.EpisodeId)
	reader, ct, err := h.proxy.ServeHLSResource(ctx, params.ServiceTag, contentKey, "hls", "steering", "")
	if err != nil {
		return nil, proxyError(err)
	}
	return episodeHLSSteeringResponse(reader, ct)
}

// ── HLS Resource Response Helpers ─────────────────────────────────────────

func movieHLSPartialResponse(r io.Reader, ct string) (oas.GetMovieHLSPartialRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetMovieHLSPartialOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetMovieHLSPartialOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetMovieHLSPartialOKVideoMp2t{Data: r}, nil
	}
}

func movieHLSPreloadHintResponse(r io.Reader, ct string) (oas.GetMovieHLSPreloadHintRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetMovieHLSPreloadHintOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetMovieHLSPreloadHintOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetMovieHLSPreloadHintOKVideoMp2t{Data: r}, nil
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

func episodeHLSPartialResponse(r io.Reader, ct string) (oas.GetEpisodeHLSPartialRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetEpisodeHLSPartialOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetEpisodeHLSPartialOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetEpisodeHLSPartialOKVideoMp2t{Data: r}, nil
	}
}

func episodeHLSPreloadHintResponse(r io.Reader, ct string) (oas.GetEpisodeHLSPreloadHintRes, error) {
	switch ct {
	case "video/mp4":
		return &oas.GetEpisodeHLSPreloadHintOKVideoMP4{Data: r}, nil
	case "video/mp2t":
		return &oas.GetEpisodeHLSPreloadHintOKVideoMp2t{Data: r}, nil
	default:
		return &oas.GetEpisodeHLSPreloadHintOKVideoMp2t{Data: r}, nil
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
