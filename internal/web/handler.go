package web

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/registry"
	"github.com/nem-git/abcmovies/internal/search"
	"github.com/nem-git/abcmovies/internal/streamfmt"
	"github.com/nem-git/abcmovies/internal/web/fragments"
	"github.com/nem-git/abcmovies/internal/web/layouts"
	"github.com/nem-git/abcmovies/internal/web/pages"
)

type Handler struct {
	registry  *registry.Registry
	mux       *http.ServeMux
	baseURL   string
	apiPrefix string
}

const defaultPageSize = 20
const maxPageSize = 100

func New(r *registry.Registry, baseURL, apiPrefix string) *Handler {
	h := &Handler{registry: r, baseURL: baseURL, apiPrefix: apiPrefix}
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static", http.FileServer(http.Dir("internal/web/static"))))

	mux.Handle("GET /", http.RedirectHandler("/services", http.StatusFound))
	mux.HandleFunc("GET /services", h.handleServiceList)
	mux.HandleFunc("GET /search", h.handleSearch)
	mux.HandleFunc("GET /services/{tag}", h.handleServiceByTag)
	mux.HandleFunc("GET /services/{tag}/movies", h.handleServiceMovies)
	mux.HandleFunc("GET /services/{tag}/movies/{id}/player", h.handleServiceMoviePlayer)
	mux.HandleFunc("GET /services/{tag}/series/{id}/seasons/{sid}/episodes/{eid}/player", h.handleServiceEpisodePlayer)
	mux.HandleFunc("GET /services/{tag}/movies/{id}", h.handleServiceMovieByID)
	mux.HandleFunc("GET /services/{tag}/series", h.handleServiceSeries)
	mux.HandleFunc("GET /services/{tag}/series/{id}", h.handleServiceSeriesByID)
	mux.HandleFunc("GET /services/{tag}/series/{id}/seasons/{sid}", h.handleServiceSeasonByID)
	mux.HandleFunc("GET /services/{tag}/series/{id}/seasons/{sid}/episodes/{eid}", h.handleServiceEpisodeByID)

	h.mux = mux
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) apiBaseURL() string {
	return h.baseURL + h.apiPrefix
}

func (h *Handler) servePage(w http.ResponseWriter, r *http.Request, c templ.Component, options ...func(*templ.ComponentHandler)) {
	templ.Handler(c, options...).ServeHTTP(w, r)
}

func (h *Handler) getPathValue(slug string, r *http.Request) (string, error) {
	v := r.PathValue(slug)
	if v == "" {
		return "", errors.New("The value of " + slug + " is invalid")
	}
	return v, nil
}

func (h *Handler) getProviders() ([]provider.Provider, error) {
	providers := h.registry.All()
	if len(providers) == 0 {
		return nil, errors.New("No streaming service providers found")
	}
	return providers, nil
}

func (h *Handler) getServices(ctx context.Context) ([]*oas.Service, error) {
	providers, err := h.getProviders()
	if err != nil {
		return nil, err
	}

	var services []*oas.Service

	for _, p := range providers {
		service, err := p.Service(ctx)
		if err == nil {
			services = append(services, service)
		} else {
			log.Printf("Error while getting service from %s: %s", p.Tag(), err)
		}
	}

	if len(services) == 0 {
		return nil, errors.New("No streaming service found")
	}

	return services, nil
}

func (h *Handler) handleServiceList(w http.ResponseWriter, r *http.Request) {
	services, err := h.getServices(r.Context())
	if err == nil {
		h.servePage(w, r, pages.ServicesList(services))
	} else {
		h.servePage(w, r, layouts.ErrorPage("Services", "We couldn't find any streaming services right now. Please try again later.", ""))
	}
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	types := parseSearchTypes(r)
	providers := h.registry.All()

	var allResults []search.Result

	for _, p := range providers {
		results, _, err := p.Search(r.Context(), query, maxPageSize, 0)
		if err != nil {
			log.Printf("Error searching %s: %s", p.Tag(), err)
			continue
		}
		for _, item := range results {
			allResults = append(allResults, search.Result{Tag: p.Tag(), Item: item})
		}
	}

	allResults = search.FilterByType(allResults, types)
	allResults = search.ScoreAndSort(query, allResults)

	if r.Header.Get("HX-Request") == "true" {
		h.servePage(w, r, fragments.SearchResults(allResults))
	} else {
		h.servePage(w, r, pages.SearchResults(query, allResults, types))
	}
}

func parseSearchTypes(r *http.Request) []string {
	raw := r.URL.Query().Get("type")
	if raw == "" {
		return []string{"movie", "series"}
	}
	parts := strings.Split(raw, ",")
	var valid []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "movie" || p == "series" || p == "service" {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return []string{"movie", "series"}
	}
	return valid
}

func (h *Handler) handleServiceByTag(w http.ResponseWriter, r *http.Request) {
	tag, err := h.getPathValue("tag", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage("Service", "We couldn't find that streaming service.", "/services"))
		return
	}

	p, err := h.registry.Get(tag)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that streaming service.", "/services"))
		return
	}

	service, err := p.Service(r.Context())
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "This streaming service is currently unavailable. Please try again later.", "/services"))
		return
	}

	h.servePage(w, r, pages.ServiceDetail(service))
}

func (h *Handler) handleServiceMovies(w http.ResponseWriter, r *http.Request) {
	tag, err := h.getPathValue("tag", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage("Movies", "We couldn't find that streaming service.", "/services"))
		return
	}

	p, err := h.registry.Get(tag)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that streaming service.", "/services"))
		return
	}

	limit, offset := parsePagination(r)
	movies, total, err := p.GetMovies(r.Context(), limit, offset)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We ran into an issue loading movies. Please try again later.", "/services/"+tag))
		return
	}

	nextURL := ""
	nextOffset := offset + limit
	if nextOffset < total {
		nextURL = "/services/" + tag + "/movies?offset=" + strconv.Itoa(nextOffset)
	}

	if r.Header.Get("HX-Request") == "true" {
		h.servePage(w, r, fragments.Movies(tag, movies, nextURL))
	} else {
		h.servePage(w, r, pages.MoviesList(tag, movies, nextURL))
	}
}

func (h *Handler) handleServiceMovieByID(w http.ResponseWriter, r *http.Request) {
	tag, err := h.getPathValue("tag", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage("Movie", "We couldn't find that streaming service.", "/services"))
		return
	}

	id, err := h.getPathValue("id", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that movie.", "/services/"+tag+"/movies"))
		return
	}

	p, err := h.registry.Get(tag)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that streaming service.", "/services/"+tag+"/movies"))
		return
	}

	movie, err := p.GetMovieById(r.Context(), id)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that movie.", "/services/"+tag+"/movies"))
		return
	}

	streams, _, err := p.GetMovieStreams(r.Context(), id)
	if err != nil {
		log.Printf("Error getting streams for movie %s/%s: %s", tag, id, err)
		streams = nil
	}

	streamURL := h.apiBaseURL() + "/services/" + tag + "/movies/" + id + "/streams"
	h.servePage(w, r, pages.MovieDetail(tag, movie, streams, streamURL))
}

func (h *Handler) handleServiceSeries(w http.ResponseWriter, r *http.Request) {
	tag, err := h.getPathValue("tag", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage("Series", "We couldn't find that streaming service.", "/services"))
		return
	}

	p, err := h.registry.Get(tag)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that streaming service.", "/services"))
		return
	}

	limit, offset := parsePagination(r)
	series, total, err := p.GetSeries(r.Context(), limit, offset)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We ran into an issue loading series. Please try again later.", "/services/"+tag))
		return
	}

	nextURL := ""
	nextOffset := offset + limit
	if nextOffset < total {
		nextURL = "/services/" + tag + "/series?offset=" + strconv.Itoa(nextOffset)
	}

	if r.Header.Get("HX-Request") == "true" {
		h.servePage(w, r, fragments.Series(tag, series, nextURL))
	} else {
		h.servePage(w, r, pages.SeriesList(tag, series, nextURL))
	}
}

func (h *Handler) handleServiceSeriesByID(w http.ResponseWriter, r *http.Request) {
	tag, err := h.getPathValue("tag", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage("Series", "We couldn't find that streaming service.", "/services"))
		return
	}

	id, err := h.getPathValue("id", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that series.", "/services/"+tag+"/series"))
		return
	}

	p, err := h.registry.Get(tag)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that streaming service.", "/services/"+tag+"/series"))
		return
	}

	limit, offset := parsePagination(r)
	seasons, total, err := p.GetSeasons(r.Context(), id, limit, offset)
	if err != nil {
		log.Printf("Error getting seasons for series %s/%s: %s", tag, id, err)
		seasons = nil
		total = 0
	}

	nextURL := ""
	nextOffset := offset + limit
	if nextOffset < total {
		nextURL = "/services/" + tag + "/series/" + id + "?offset=" + strconv.Itoa(nextOffset)
	}

	if r.Header.Get("HX-Request") == "true" {
		h.servePage(w, r, fragments.Seasons(tag, id, seasons, nextURL))
		return
	}

	series, err := p.GetSeriesByID(r.Context(), id)
	if err != nil {
		log.Printf("seriesByID: %s", err.Error())
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that series.", "/services/"+tag+"/series"))
		return
	}

	h.servePage(w, r, pages.SeriesDetail(tag, series, seasons, nextURL))
}

func (h *Handler) handleServiceSeasonByID(w http.ResponseWriter, r *http.Request) {
	tag, err := h.getPathValue("tag", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage("Season", "We couldn't find that streaming service.", "/services"))
		return
	}

	id, err := h.getPathValue("id", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that series.", "/services/"+tag+"/series"))
		return
	}

	sid, err := h.getPathValue("sid", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that season.", "/services/"+tag+"/series/"+id))
		return
	}

	p, err := h.registry.Get(tag)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that streaming service.", "/services/"+tag+"/series/"+id))
		return
	}

	limit, offset := parsePagination(r)
	episodes, total, err := p.GetEpisodes(r.Context(), id, sid, limit, offset)
	if err != nil {
		log.Printf("Error getting episodes for season %s/%s/%s: %s", tag, id, sid, err)
		episodes = nil
		total = 0
	}

	nextURL := ""
	nextOffset := offset + limit
	if nextOffset < total {
		nextURL = "/services/" + tag + "/series/" + id + "/seasons/" + sid + "?offset=" + strconv.Itoa(nextOffset)
	}

	if r.Header.Get("HX-Request") == "true" {
		h.servePage(w, r, fragments.Episodes(tag, id, sid, episodes, nextURL))
		return
	}

	season, err := p.GetSeasonById(r.Context(), id, sid)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that season.", "/services/"+tag+"/series/"+id))
		return
	}

	h.servePage(w, r, pages.SeasonDetail(tag, id, season, episodes, nextURL))
}

func (h *Handler) handleServiceEpisodeByID(w http.ResponseWriter, r *http.Request) {
	tag, err := h.getPathValue("tag", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage("Episode", "We couldn't find that streaming service.", "/services"))
		return
	}

	id, err := h.getPathValue("id", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that series.", "/services/"+tag+"/series"))
		return
	}

	sid, err := h.getPathValue("sid", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that season.", "/services/"+tag+"/series/"+id))
		return
	}

	eid, err := h.getPathValue("eid", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that episode.", "/services/"+tag+"/series/"+id+"/seasons/"+sid))
		return
	}

	p, err := h.registry.Get(tag)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that streaming service.", "/services/"+tag+"/series/"+id+"/seasons/"+sid))
		return
	}

	episode, err := p.GetEpisodeById(r.Context(), id, sid, eid)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that episode.", "/services/"+tag+"/series/"+id+"/seasons/"+sid))
		return
	}

	streams, _, err := p.GetEpisodeStreams(r.Context(), id, sid, eid)
	if err != nil {
		log.Printf("Error getting streams for episode %s/%s/%s/%s: %s", tag, id, sid, eid, err)
		streams = nil
	}

	streamURL := h.apiBaseURL() + "/services/" + tag + "/series/" + id + "/seasons/" + sid + "/episodes/" + eid + "/streams"
	h.servePage(w, r, pages.EpisodeDetail(tag, id, sid, episode, streams, streamURL))
}

func (h *Handler) handleServiceMoviePlayer(w http.ResponseWriter, r *http.Request) {
	tag, err := h.getPathValue("tag", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage("Player", "We couldn't find that streaming service.", "/services"))
		return
	}

	id, err := h.getPathValue("id", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that movie.", "/services/"+tag+"/movies"))
		return
	}

	p, err := h.registry.Get(tag)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that streaming service.", "/services/"+tag+"/movies"))
		return
	}

	movie, err := p.GetMovieById(r.Context(), id)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that movie.", "/services/"+tag+"/movies"))
		return
	}

	streams, _, err := p.GetMovieStreams(r.Context(), id)
	if err != nil || len(streams) == 0 {
		h.servePage(w, r, layouts.ErrorPage(tag, "No streams available for this movie.", "/services/"+tag+"/movies/"+id))
		return
	}

	best := pickBestStream(streams)
	if best == nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "No compatible streams available.", "/services/"+tag+"/movies/"+id))
		return
	}

	streamURL := h.apiBaseURL() + "/services/" + tag + "/movies/" + id + "/streams/" + streamfmt.ShortName(best.EncodingFormat) + "/" + best.ID
	h.servePage(w, r, pages.MoviePlayer(tag, id, movie, streamURL))
}

func (h *Handler) handleServiceEpisodePlayer(w http.ResponseWriter, r *http.Request) {
	tag, err := h.getPathValue("tag", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage("Player", "We couldn't find that streaming service.", "/services"))
		return
	}

	id, err := h.getPathValue("id", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that series.", "/services/"+tag+"/series"))
		return
	}

	sid, err := h.getPathValue("sid", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that season.", "/services/"+tag+"/series/"+id))
		return
	}

	eid, err := h.getPathValue("eid", r)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that episode.", "/services/"+tag+"/series/"+id+"/seasons/"+sid))
		return
	}

	p, err := h.registry.Get(tag)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that streaming service.", "/services/"+tag+"/series/"+id+"/seasons/"+sid))
		return
	}

	episode, err := p.GetEpisodeById(r.Context(), id, sid, eid)
	if err != nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "We couldn't find that episode.", "/services/"+tag+"/series/"+id+"/seasons/"+sid))
		return
	}

	streams, _, err := p.GetEpisodeStreams(r.Context(), id, sid, eid)
	if err != nil || len(streams) == 0 {
		h.servePage(w, r, layouts.ErrorPage(tag, "No streams available for this episode.", "/services/"+tag+"/series/"+id+"/seasons/"+sid+"/episodes/"+eid))
		return
	}

	best := pickBestStream(streams)
	if best == nil {
		h.servePage(w, r, layouts.ErrorPage(tag, "No compatible streams available.", "/services/"+tag+"/series/"+id+"/seasons/"+sid+"/episodes/"+eid))
		return
	}

	streamURL := h.apiBaseURL() + "/services/" + tag + "/series/" + id + "/seasons/" + sid + "/episodes/" + eid + "/streams/" + streamfmt.ShortName(best.EncodingFormat) + "/" + best.ID
	h.servePage(w, r, pages.EpisodePlayer(tag, id, sid, episode, streamURL))
}

func pickBestStream(streams []oas.Stream) *oas.Stream {
	for _, s := range streams {
		if s.EncodingFormat == oas.StreamEncodingFormatApplicationDashXML {
			return &s
		}
	}
	for _, s := range streams {
		if s.EncodingFormat == oas.StreamEncodingFormatApplicationVndAppleMpegurl {
			return &s
		}
	}
	if len(streams) > 0 {
		return &streams[0]
	}
	return nil
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultPageSize
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= maxPageSize {
			limit = n
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	return limit, offset
}
