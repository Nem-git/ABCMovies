package cbc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/providers/cbc/types"
	"golang.org/x/text/language"
)

type Config struct {
	Tag     string
	Service *oas.Service

	// BaseURL overrides the API endpoints for testing. Empty in production.
	BaseURL string
	// APIPrefix is the app's API prefix.
	APIPrefix string
}

type Provider struct {
	provider.UnimplementedProvider
	cfg Config
	cli *client
}

func New(cfg Config) *Provider {
	opts := clientOptions{}
	if cfg.BaseURL != "" {
		opts.baseURL = cfg.BaseURL
	}
	return &Provider{
		cfg: cfg,
		cli: newClientWithOpts(opts),
	}
}

func (p *Provider) imageURL(parts ...string) oas.OptURI {
	path := strings.Join(parts, "/")
	raw := p.cfg.BaseURL + p.cfg.APIPrefix + "/services/" + p.cfg.Tag + "/" + path
	u, err := url.Parse(raw)
	if err != nil {
		return oas.OptURI{}
	}
	return oas.NewOptURI(*u)
}

func (p *Provider) Tag() string {
	return p.cfg.Tag
}

func (p *Provider) Service(ctx context.Context) (*oas.Service, error) {
	if p.cfg.Service == nil {
		return p.UnimplementedProvider.Service(ctx)
	}
	return p.cfg.Service, nil
}

func (p *Provider) Health(ctx context.Context) (*oas.Health, error) {
	err := p.cli.getBrowse(ctx)
	if err != nil {
		return &oas.Health{Status: oas.HealthStatusDown}, nil
	}
	return &oas.Health{Status: oas.HealthStatusOk}, nil
}

// ── Series ─────────────────────────────────────────────────────────────────

func (p *Provider) GetSeries(ctx context.Context, limit, offset int) ([]oas.Series, int, error) {
	f, err := p.cli.getCategory(ctx, "shows", pageFor(offset, limit), pageSizeFor(limit))
	if err != nil {
		return nil, 0, fmt.Errorf("fetching series list: %w", err)
	}
	pr := getPageResults(f)
	total := totalRecords(f)
	items := make([]oas.Series, 0, len(pr))
	for _, r := range pr {
		items = append(items, p.mapSeriesFromSearch(r))
	}
	return items, total, nil
}

func (p *Provider) GetSeriesByID(ctx context.Context, seriesID string) (*oas.Series, error) {
	sr, err := p.cli.getShow(ctx, seriesID)
	if err != nil {
		return nil, fmt.Errorf("fetching series %q: %w", seriesID, err)
	}
	return p.mapSeriesFromShow(sr, seriesID)
}

func (p *Provider) GetSeriesPoster(ctx context.Context, seriesID string) (io.ReadCloser, string, error) {
	return p.GetMoviePoster(ctx, seriesID)
}

func (p *Provider) GetSeriesBackdrop(ctx context.Context, seriesID string) (io.ReadCloser, string, error) {
	return p.GetMovieBackdrop(ctx, seriesID)
}

// ── Seasons ────────────────────────────────────────────────────────────────

func (p *Provider) GetSeasons(ctx context.Context, seriesID string, limit, offset int) ([]oas.Season, int, error) {
	sr, err := p.cli.getShow(ctx, seriesID)
	if err != nil {
		return nil, 0, fmt.Errorf("fetching seasons for series %q: %w", seriesID, err)
	}
	lineups := extractLineups(sr)
	if lineups == nil {
		return nil, 0, provider.ErrNotSupported
	}
	seasons := make([]oas.Season, 0, len(lineups))
	for _, lu := range lineups {
		seasons = append(seasons, p.mapSeason(lu, sr.Images, seriesID))
	}
	return paginate(seasons, limit, offset)
}

func (p *Provider) GetSeasonById(ctx context.Context, seriesID, seasonID string) (*oas.Season, error) {
	num, err := parseSeasonNumber(seasonID)
	if err != nil {
		return nil, fmt.Errorf("invalid season ID %q: %w", seasonID, err)
	}
	sr, err := p.cli.getShow(ctx, seriesID)
	if err != nil {
		return nil, fmt.Errorf("fetching season for series %q: %w", seriesID, err)
	}
	lineups := extractLineups(sr)
	for _, lu := range lineups {
		if lu.SeasonNumber == num {
			s := p.mapSeason(lu, sr.Images, seriesID)
			return &s, nil
		}
	}
	return nil, provider.ErrNotSupported
}

func (p *Provider) GetSeasonPoster(ctx context.Context, seriesID, seasonID string) (io.ReadCloser, string, error) {
	return p.GetSeriesPoster(ctx, seriesID)
}

func (p *Provider) GetSeasonBackdrop(ctx context.Context, seriesID, seasonID string) (io.ReadCloser, string, error) {
	return p.GetSeriesBackdrop(ctx, seriesID)
}

// ── Episodes ───────────────────────────────────────────────────────────────

func (p *Provider) GetEpisodes(ctx context.Context, seriesID, seasonID string, limit, offset int) ([]oas.Episode, int, error) {
	num, err := parseSeasonNumber(seasonID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid season ID %q: %w", seasonID, err)
	}
	sr, err := p.cli.getShow(ctx, seriesID)
	if err != nil {
		return nil, 0, fmt.Errorf("fetching episodes for series %q: %w", seriesID, err)
	}
	lineups := extractLineups(sr)
	for _, lu := range lineups {
		if lu.SeasonNumber == num {
			eps := make([]oas.Episode, 0, len(lu.Items))
			for _, item := range lu.Items {
				eps = append(eps, p.mapEpisode(item, seriesID, seasonID))
			}
			return paginate(eps, limit, offset)
		}
	}
	return nil, 0, provider.ErrNotSupported
}

func (p *Provider) GetEpisodeById(ctx context.Context, seriesID, seasonID, episodeID string) (*oas.Episode, error) {
	num, err := parseSeasonNumber(seasonID)
	if err != nil {
		return nil, fmt.Errorf("invalid season ID %q: %w", seasonID, err)
	}
	sr, err := p.cli.getShow(ctx, seriesID)
	if err != nil {
		return nil, fmt.Errorf("fetching episode for series %q: %w", seriesID, err)
	}
	lineups := extractLineups(sr)
	for _, lu := range lineups {
		if lu.SeasonNumber == num {
			for _, item := range lu.Items {
				if strconv.Itoa(item.IDMedia) == episodeID {
					ep := p.mapEpisode(item, seriesID, seasonID)
					return &ep, nil
				}
			}
		}
	}
	return nil, provider.ErrNotSupported
}

func (p *Provider) GetEpisodeStreams(ctx context.Context, _, _, episodeID string) ([]oas.Stream, int, error) {
	meta, err := p.cli.getStreamMeta(ctx, episodeID, "gem", "")
	if err != nil {
		return nil, 0, fmt.Errorf("fetching stream info for episode %q: %w", episodeID, err)
	}
	if meta.ErrorMessage != nil {
		return nil, 0, fmt.Errorf("media meta error: code=%d text=%s", meta.ErrorMessage.ErrorCode, meta.ErrorMessage.Text)
	}
	var streams []oas.Stream
	for _, t := range meta.AvailableTechs {
		switch t.Name {
		case "dash":
			streams = append(streams, oas.Stream{
				Type:           oas.StreamTypeVideoObject,
				ID:             "manifest.mpd",
				Name:           "DASH Stream",
				EncodingFormat: oas.StreamEncodingFormatApplicationDashXML,
			})
		case "hls":
			streams = append(streams, oas.Stream{
				Type:           oas.StreamTypeVideoObject,
				ID:             "master.m3u8",
				Name:           "HLS Stream",
				EncodingFormat: oas.StreamEncodingFormatApplicationVndAppleMpegurl,
			})
		}
	}
	return paginate(streams, len(streams), 0)
}

func (p *Provider) GetEpisodeStreamFile(ctx context.Context, _, _, episodeID, streamFile string) (io.ReadCloser, string, error) {
	tech, err := streamFileToTech(streamFile)
	if err != nil {
		return nil, "", err
	}
	val, err := p.cli.getStreamValidation(ctx, episodeID, "gem", tech)
	if err != nil {
		return nil, "", fmt.Errorf("fetching stream URL for episode %q: %w", episodeID, err)
	}
	mime := dashMIME
	if tech == "hls" {
		mime = hlsMIME
	}
	rc, _, err := p.cli.getRaw(ctx, val.URL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("fetching stream manifest: %w", err)
	}
	return rc, mime, nil
}

func (p *Provider) GetEpisodeSubtitles(context.Context, string, string, string) ([]oas.Subtitle, int, error) {
	return nil, 0, provider.ErrNotSupported
}

func (p *Provider) GetEpisodeSubtitleFile(context.Context, string, string, string, string) (io.ReadCloser, string, error) {
	return nil, "", provider.ErrNotSupported
}

func (p *Provider) GetEpisodeThumbnail(ctx context.Context, seriesID, seasonID, episodeID string) (io.ReadCloser, string, error) {
	num, err := parseSeasonNumber(seasonID)
	if err != nil {
		return nil, "", fmt.Errorf("invalid season ID %q: %w", seasonID, err)
	}
	sr, err := p.cli.getShow(ctx, seriesID)
	if err != nil {
		return nil, "", fmt.Errorf("fetching thumbnail for series %q: %w", seriesID, err)
	}
	lineups := extractLineups(sr)
	for _, lu := range lineups {
		if lu.SeasonNumber == num {
			for _, item := range lu.Items {
				if strconv.Itoa(item.IDMedia) == episodeID && item.Images != nil {
					im := p.getImageThumbnail(item.Images)
					if im == "" {
						return nil, "", provider.ErrNotSupported
					}

					return p.cli.getImage(ctx, im)
				}
			}
		}
	}
	return nil, "", provider.ErrNotSupported
}

// ── Movies ─────────────────────────────────────────────────────────────────

func (p *Provider) GetMovies(ctx context.Context, limit, offset int) ([]oas.Movie, int, error) {
	f, err := p.cli.getCategory(ctx, "all-films", pageFor(offset, limit), pageSizeFor(limit))
	if err != nil {
		return nil, 0, fmt.Errorf("fetching movies list: %w", err)
	}
	pr := getPageResults(f)
	total := totalRecords(f)
	items := make([]oas.Movie, 0, len(pr))
	for _, r := range pr {
		items = append(items, p.mapMovieFromSearch(r))
	}
	return items, total, nil
}

func (p *Provider) GetMovieById(ctx context.Context, movieId string) (*oas.Movie, error) {
	sr, err := p.cli.getShow(ctx, movieId)
	if err != nil {
		return nil, fmt.Errorf("fetching movie %q: %w", movieId, err)
	}
	return p.mapMovieFromShow(sr, movieId)
}

func (p *Provider) GetMovieStreams(context.Context, string) ([]oas.Stream, int, error) {
	return nil, 0, provider.ErrNotSupported
}

func (p *Provider) GetMovieStreamFile(context.Context, string, string) (io.ReadCloser, string, error) {
	return nil, "", provider.ErrNotSupported
}

func (p *Provider) GetMovieSubtitles(context.Context, string) ([]oas.Subtitle, int, error) {
	return nil, 0, provider.ErrNotSupported
}

func (p *Provider) GetMovieSubtitleFile(context.Context, string, string) (io.ReadCloser, string, error) {
	return nil, "", provider.ErrNotSupported
}

func (p *Provider) GetMoviePoster(ctx context.Context, movieId string) (io.ReadCloser, string, error) {
	log.Println("ASDASd")
	return p.fetchShowImage(ctx, movieId, func(im *types.Images) string {
		if im == nil {
			return ""
		}

		log.Println(im)

		if im.Card != nil {
			return im.Card.URL
		}
		if im.Background != nil {
			return im.Background.URL
		}

		return ""
	})
}

func (p *Provider) GetMovieBackdrop(ctx context.Context, movieId string) (io.ReadCloser, string, error) {
	return p.fetchShowImage(ctx, movieId, func(im *types.Images) string {
		if im == nil {
			return ""
		}

		if im.Background != nil {
			return im.Background.URL
		}
		if im.Card != nil {
			return im.Card.URL
		}

		return ""
	})
}

// ── Search ─────────────────────────────────────────────────────────────────

func (p *Provider) Search(ctx context.Context, query string, limit, offset int) ([]oas.SearchResultItem, int, error) {
	resp, err := p.cli.search(ctx, query, 0, 0)
	if err != nil {
		return nil, 0, fmt.Errorf("searching %q: %w", query, err)
	}
	var results []types.Result
	if resp.Results != nil {
		results = *resp.Results
	}
	items := make([]oas.SearchResultItem, 0, len(results))
	for _, r := range results {
		item := p.mapSearchResult(r)
		if item != nil {
			items = append(items, *item)
		}
	}
	return paginate(items, limit, offset)
}

// ── Helpers ────────────────────────────────────────────────────────────────

func getPageResults(resp *types.CategoryResponse) []types.Result {
	var results []types.Result
	if resp.Contents != nil {
		for _, c := range resp.Contents {
			if c.Items != nil && c.Items.Results != nil {
				for _, r := range *c.Items.Results {
					results = append(results, r)
				}
			}
		}
	}
	return results
}

func totalRecords(resp *types.CategoryResponse) int {
	if len(resp.Contents) > 0 && resp.Contents[0].Items != nil {
		return resp.Contents[0].Items.TotalRecords
	}
	return 0
}

func paginate[T any](items []T, limit, offset int) ([]T, int, error) {
	n := len(items)
	if n == 0 || offset >= n || limit <= 0 {
		if n == 0 {
			return nil, 0, nil
		}
		return nil, n, nil
	}
	end := min(offset+limit, n)
	return items[offset:end], n, nil
}

func extractLineups(sr *types.ShowResponse) []types.Lineup {
	for _, c := range sr.Contents {
		if len(c.Lineups) > 0 {
			return c.Lineups
		}
	}
	return nil
}

func (p *Provider) mapSeriesFromSearch(r types.Result) oas.Series {
	s := oas.Series{
		Type:            oas.SeriesTypeTVSeries,
		ID:              r.URL,
		Name:            r.Title,
		NumberOfSeasons: parseInfoNumberOfSeasons(r.InfoTitle),
	}
	if r.Description != "" {
		s.Description = oas.NewOptString(r.Description)
	}
	if genre := parseInfoGenre(r.InfoTitle); genre != "" {
		s.Genres = append(s.Genres, genre)
	}
	if r.Images != nil && p.getImagePoster(r.Images) != "" {
		s.Poster = p.imageURL("series", r.URL, "poster")
	}
	if r.Images != nil && p.getImageBackdrop(r.Images) != "" {
		s.Backdrop = p.imageURL("series", r.URL, "backdrop")
	}

	return s
}

func (p *Provider) mapSeriesFromShow(sr *types.ShowResponse, seriesID string) (*oas.Series, error) {
	s := oas.Series{
		Type: oas.SeriesTypeTVSeries,
		ID:   seriesID,
		Name: sr.Title,
	}
	if sr.Description != "" {
		s.Description = oas.NewOptString(sr.Description)
	}
	if sr.StructuredMetadata != nil {
		if !sr.StructuredMetadata.DatePublished.IsZero() {
			s.DatePublished = oas.NewOptDate(sr.StructuredMetadata.DatePublished)
		}
		if tag, err := language.BCP47.Parse(sr.StructuredMetadata.InLanguage); err == nil {
			b, _ := tag.Base()
			s.Languages = append(s.Languages, b.String())
		}
		if sr.StructuredMetadata.ContentRating != "" {
			s.ContentRating = oas.NewOptString(sr.StructuredMetadata.ContentRating)
		}
		if len(sr.StructuredMetadata.Genres) > 0 {
			s.Genres = append(s.Genres, sr.StructuredMetadata.Genres...)
		}
	}

	s.NumberOfSeasons = countLineups(sr)
	if sr.Images != nil && p.getImagePoster(sr.Images) != "" {
		s.Poster = p.imageURL("series", seriesID, "poster")
	}
	if sr.Images != nil && p.getImageBackdrop(sr.Images) != "" {
		s.Backdrop = p.imageURL("series", seriesID, "backdrop")
	}
	return &s, nil
}

func countLineups(sr *types.ShowResponse) int {
	for _, c := range sr.Contents {
		return len(c.Lineups)
	}
	return 0
}

func (p *Provider) mapSeason(lu types.Lineup, images *types.Images, seriesID string) oas.Season {
	s := oas.Season{
		Type: oas.SeasonTypeTVSeason,
		ID:   seasonID(lu.SeasonNumber),
		Name: lu.Title,
	}
	if lu.SeasonNumber > 0 {
		s.SeasonNumber = oas.NewOptInt(lu.SeasonNumber)
	}
	if len(lu.Items) > 0 {
		s.NumberOfEpisodes = oas.NewOptInt(len(lu.Items))
	}
	if images != nil && images.Card != nil {
		s.Poster = p.imageURL("series", seriesID, "seasons", s.ID, "poster")
	}
	if images != nil && images.Background != nil {
		s.Backdrop = p.imageURL("series", seriesID, "seasons", s.ID, "backdrop")
	}
	return s
}

func (p *Provider) mapEpisode(item types.Item, seriesID, seasonID string) oas.Episode {
	e := oas.Episode{
		Type: oas.EpisodeTypeTVEpisode,
		ID:   strconv.Itoa(item.IDMedia),
		Name: item.Title,
	}
	if item.Description != "" {
		e.Description = oas.NewOptString(item.Description)
	}
	if item.EpisodeNumber > 0 {
		e.EpisodeNumber = oas.NewOptInt(item.EpisodeNumber)
	}
	if item.Metadata != nil {
		if t, err := time.Parse(time.DateOnly, item.Metadata.AirDate); err == nil {
			e.DatePublished = oas.NewOptDate(t)
		}
		if item.Metadata.Duration > 0 {
			e.Duration = parseDuration(item.Metadata.Duration)
		}
		if item.Metadata.Country != "" {
			e.CountryOfOrigin = oas.NewOptString(item.Metadata.Country)
		}
		if item.Metadata.Rating != "" {
			e.ContentRating = oas.NewOptString(item.Metadata.Rating)
		}
	}
	if item.Images != nil && p.getImageThumbnail(item.Images) != "" {
		e.Poster = p.imageURL("series", seriesID, "seasons", seasonID, "episodes", e.ID, "thumbnail")
	}
	return e
}

func (p *Provider) mapMovieFromSearch(r types.Result) oas.Movie {
	m := oas.Movie{
		Type: oas.MovieTypeMovie,
		ID:   r.URL,
		Name: r.Title,
	}
	if r.Description != "" {
		m.Description = oas.NewOptString(r.Description)
	}

	// Get info from InfoTitle
	if dur := parseInfoDuration(r.InfoTitle); dur != "" {
		m.Duration = oas.NewOptString(dur)
	}
	if genre := parseInfoGenre(r.InfoTitle); genre != "" {
		m.Genres = append(m.Genres, genre)
	}
	if r.Images != nil && p.getImagePoster(r.Images) != "" {
		m.Poster = p.imageURL("movies", r.URL, "poster")
	}
	if r.Images != nil && p.getImageBackdrop(r.Images) != "" {
		m.Backdrop = p.imageURL("movies", r.URL, "backdrop")
	}

	return m
}

func (p *Provider) mapMovieFromShow(sr *types.ShowResponse, movieId string) (*oas.Movie, error) {
	if sr.StructuredMetadata != nil {
		if sr.StructuredMetadata.AtType != "Movie" {
			return nil, errors.New("The requested content is not a movie")
		}
	}

	m := oas.Movie{
		Type: oas.MovieTypeMovie,
		ID:   movieId,
		Name: sr.Title,
	}
	if sr.Description != "" {
		m.Description = oas.NewOptString(sr.Description)
	}
	if sr.StructuredMetadata != nil && sr.StructuredMetadata.Duration != "" {
		m.Duration = oas.NewOptString(sr.StructuredMetadata.Duration)
	}
	if sr.Images != nil {
		m.Poster = p.imageURL("movies", movieId, "poster")
	}
	if sr.Images != nil {
		m.Backdrop = p.imageURL("movies", movieId, "backdrop")
	}

	return &m, nil
}

func (p *Provider) mapSearchResult(r types.Result) *oas.SearchResultItem {
	item := &oas.SearchResultItem{}
	switch r.Type {
	case "Show":
		// If duration found in infotitle, must be a movie
		// Else, just think it is a series for now
		if parseInfoDuration(r.InfoTitle) == "" {
			s := p.mapSeriesFromSearch(r)
			item.Resource = oas.SearchResultItemResource{
				Type:   oas.SeriesSearchResultItemResource,
				Series: s,
			}
		} else {
			m := p.mapMovieFromSearch(r)
			item.Resource = oas.SearchResultItemResource{
				Type:  oas.MovieSearchResultItemResource,
				Movie: m,
			}
		}

	case "Media", "Live":
		m := p.mapMovieFromSearch(r)
		item.Resource = oas.SearchResultItemResource{
			Type:  oas.MovieSearchResultItemResource,
			Movie: m,
		}
	default:
		return nil
	}
	return item
}

// ── Small helpers ──────────────────────────────────────────────────────────

func seasonID(n int) string {
	return fmt.Sprintf("s%02d", n)
}

func parseSeasonNumber(id string) (int, error) {
	if !strings.HasPrefix(id, "s") {
		return 0, fmt.Errorf("invalid season ID %q", id)
	}
	return strconv.Atoi(id[1:])
}

func streamFileToTech(filename string) (string, error) {
	switch filename {
	case "manifest.mpd":
		return "dash", nil
	case "master.m3u8":
		return "hls", nil
	default:
		return "", fmt.Errorf("unknown stream file %q", filename)
	}
}

func parseDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		if s > 0 {
			return fmt.Sprintf("PT%dH%dM%dS", h, m, s)
		}
		if m > 0 {
			return fmt.Sprintf("PT%dH%dM", h, m)
		}
		return fmt.Sprintf("PT%dH", h)
	}
	if s > 0 {
		return fmt.Sprintf("PT%dM%dS", m, s)
	}
	return fmt.Sprintf("PT%dM", m)
}

func parseInfoNumberOfSeasons(infoTitle string) int {
	defaultNumberOfSeason := 1

	parts := strings.Split(infoTitle, "|")
	if len(parts) < 2 {
		return defaultNumberOfSeason
	}
	// Can be season, seasons, episodes, maybe more!
	// Might look like:
	// 1 season
	// 2 seasons
	// 21 episodes
	// 2 parts
	// 45 min
	// more!!
	numberOfMediaStr := strings.TrimSpace(parts[len(parts)-1])
	tokens := strings.Fields(numberOfMediaStr)
	if len(tokens) != 2 {
		return defaultNumberOfSeason
	}

	switch tokens[1] {
	case "season":
		return defaultNumberOfSeason
	case "seasons":
		{
			numberOfSeasons, err := strconv.Atoi(tokens[0])
			if err == nil {
				return numberOfSeasons
			}
		}
	}

	return defaultNumberOfSeason
}

func parseInfoGenre(infoTitle string) string {
	parts := strings.Split(infoTitle, " ")
	if len(parts) == 1 {
		return infoTitle
	}
	return ""
}

func parseInfoDuration(infoTitle string) string {
	parts := strings.Split(infoTitle, "|")
	if len(parts) < 2 {
		return ""
	}
	durStr := strings.TrimSpace(parts[len(parts)-1])
	tokens := strings.Fields(durStr)
	var total int
	for i := 0; i < len(tokens); i++ {
		n, err := strconv.Atoi(tokens[i])
		if err != nil {
			continue
		}
		if i+1 < len(tokens) {
			switch tokens[i+1] {
			case "h":
				total += n * 60
				i++
			case "min":
				total += n
				i++
			}
		}
	}
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("PT%dM", total)
}

func pageFor(offset, limit int) int {
	if limit <= 0 {
		return 1
	}
	return offset/limit + 1
}

func pageSizeFor(limit int) int {
	if limit <= 0 {
		return defaultPageSize
	}
	return limit
}

func (p *Provider) fetchShowImage(ctx context.Context, showId string, selector func(*types.Images) string) (io.ReadCloser, string, error) {
	sr, err := p.cli.getShow(ctx, showId)
	if err != nil {
		return nil, "", err
	}

	// Takes the image of the show
	if url := selector(sr.Images); url != "" {
		return p.cli.getImage(ctx, url)
	}

	// Takes the image of the first piece of content in the show
	for _, c := range sr.Contents {
		if c.Lineups != nil {
			for _, lu := range c.Lineups {
				if lu.Items != nil {
					for _, item := range lu.Items {
						if url := selector(item.Images); url != "" {
							return p.cli.getImage(ctx, url)
						}
					}
				}
			}
		}
	}

	return nil, "", provider.ErrNotSupported
}

// Selects the less bad option
func (p *Provider) getImageBackdrop(im *types.Images) string {
	if im == nil {
		return ""
	}

	if im.Background != nil {
		return im.Background.URL
	}
	if im.Card != nil {
		return im.Card.URL
	}

	return ""
}

// Selects the less bad option
func (p *Provider) getImagePoster(im *types.Images) string {
	if im == nil {
		return ""
	}

	if im.Card != nil {
		return im.Card.URL
	}
	if im.Background != nil {
		return im.Background.URL
	}

	return ""
}

// Selects the less bad option
func (p *Provider) getImageThumbnail(im *types.Images) string {
	if im == nil {
		return ""
	}

	if im.Card != nil {
		return im.Card.URL
	}
	if im.Background != nil {
		return im.Background.URL
	}

	return ""
}

const (
	defaultPageSize = 20
	dashMIME        = "application/dash+xml"
	hlsMIME         = "application/vnd.apple.mpegurl"
)
