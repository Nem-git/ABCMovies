// Package tmdb implements the catalogue slot contract against The Movie
// Database API v3 (TECHNICAL-DECISIONS.md §1.27): text lookup for the
// enrichment fallback, full records keyed on the native tmdb:{id} ref, and
// foreign-ID resolution through the /find bridge. Like every adapter it
// stays pure — HTTP plus generated protos only; wiring lives in the core's
// slotwiring package. Pacing keeps requests well under the soft rate limit;
// a 429 backs off once per Retry-After before failing.
package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

const (
	defaultBase     = "https://api.themoviedb.org/3"
	defaultPace     = 200 * time.Millisecond // ~5 req/s ceiling (§1.27)
	language        = "en-US"
	imageBase       = "https://image.tmdb.org/t/p/"
	posterSize       = "w500"
	maxCandidates   = 5
	maxCast          = 5
	maxBackoff       = 30 * time.Second
)

// Slot is one TMDB catalogue instance.
type Slot struct {
	token string // v3 API key or v4 read-access bearer token
	base  string
	hc     *http.Client
	pace <-chan time.Time
}

// Option customizes the slot at construction.
type Option func(*Slot)

// WithHTTPClient replaces the outbound HTTP client (tests inject canned
// transports here; production never touches this).
func WithHTTPClient(hc *http.Client) Option {
	return func(s *Slot) { s.hc = hc }
}

// WithBaseURL points the adapter at another API root.
func WithBaseURL(base string) Option {
	return func(s *Slot) { s.base = strings.TrimRight(base, "/") }
}

// WithPace overrides the inter-request spacing (tests only; production uses
// the declared default).
func WithPace(d time.Duration) Option {
	return func(s *Slot) { s.pace = ticker(d) }
}

func ticker(d time.Duration) <-chan time.Time {
	if d <= 0 {
		d = time.Nanosecond
	}
	return time.Tick(d) // lives as long as the process; slots are never rebuilt per request
}

// New builds a slot reading its credential from the named environment
// variable at composition time — values never live in config files
// (TECHNICAL-DECISIONS.md §1.27). A missing variable aborts startup loudly.
func New(tokenEnv string, opts ...Option) (*Slot, error) {
	if tokenEnv == "" {
		return nil, fmt.Errorf("tmdb: no credential environment variable configured")
	}
	token := os.Getenv(tokenEnv)
	if token == "" {
		return nil, fmt.Errorf("tmdb: environment variable %q is not set", tokenEnv)
	}
	s := &Slot{
		token: token,
		base:  defaultBase,
		hc:     &http.Client{Timeout: 30 * time.Second},
		pace: ticker(defaultPace),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// CapabilityQuery answers the meta-contract: the slot speaks the catalogue
// capabilities whose fixture suites it passes (contract header; PLAN.md
// §2.5: nothing assumed, everything asked, nothing declared untested).
func (s *Slot) CapabilityQuery(_ context.Context, _ *corev1.CapabilityQueryRequest) (*corev1.CapabilityQueryResponse, error) {
	return &corev1.CapabilityQueryResponse{
		Capabilities: []*corev1.Capability{
			{Name: "meta", Version: 1},
			{Name: "lookup-title", Version: 1},
			{Name: "get-metadata", Version: 1},
		},
	}, nil
}

// Namespaces declares every identity namespace this adapter can resolve,
// including foreign ones via the /find bridge. The composition enforces that
// two enabled catalogue slots never claim the same namespace
// (TECHNICAL-DECISIONS.md §1.29).
func (s *Slot) Namespaces() []string { return []string{"tmdb", "imdb", "tvdb"} }

// LookupTitle maps the contract's text-lookup fallback onto TMDB search.
func (s *Slot) LookupTitle(ctx context.Context, req *slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
	q := strings.TrimSpace(req.GetQuery())
	if q == "" {
		return nil, fmt.Errorf("tmdb: empty lookup query")
	}
	media, params := "multi", url.Values{"query": {q}, "language": {language}}
	y := req.GetYear()
	switch req.GetKind() {
	case slotsv1.ItemKind_ITEM_KIND_MOVIE:
		media = "movie"
		if y > 0 {
			params.Set("year", strconv.FormatUint(uint64(y), 10))
		}
	case slotsv1.ItemKind_ITEM_KIND_SERIES:
		media = "tv"
		if y > 0 {
			params.Set("first_air_date_year", strconv.FormatUint(uint64(y), 10))
		}
	}

	var page struct {
		Results []searchResult `json:"results"`
	}
	if err := s.get(ctx, "/search/"+media, params, &page); err != nil {
		return nil, fmt.Errorf("tmdb: search %q: %w", q, err)
	}
	resp := &slotsv1.LookupTitleResponse{}
	for _, r := range page.Results {
		c, ok := r.candidate(media)
		if !ok {
			continue // multi search also returns people; skip them
		}
		resp.Candidates = append(resp.Candidates, c)
		if len(resp.Candidates) == maxCandidates {
			break
		}
	}
	return resp, nil
}

type searchResult struct {
	ID            int    `json:"id"`
	MediaType     string `json:"media_type,omitempty"`
	Title         string `json:"title,omitempty"`
	Name           string `json:"name,omitempty"`
	OriginalTitle string `json:"original_title,omitempty"`
	OriginalName  string `json:"original_name,omitempty"`
	ReleaseDate   string `json:"release_date,omitempty"`
	FirstAirDate string `json:"first_air_date,omitempty"`
	IMDbID         string `json:"imdb_id,omitempty"`
}

// resultKind resolves a hit's kind: single-media searches imply it from
// the endpoint; multi hits carry it per result. Unknown media types
// (people in multi results) are rejected.
func resultKind(searched string, hit searchResult) (slotsv1.ItemKind, bool) {
	mt := hit.MediaType
	if mt == "" {
		mt = searched // single-media search
	}
	switch mt {
	case "movie":
		return slotsv1.ItemKind_ITEM_KIND_MOVIE, true
	case "tv":
		return slotsv1.ItemKind_ITEM_KIND_SERIES, true
	default:
		return slotsv1.ItemKind_ITEM_KIND_UNSPECIFIED, false
	}
}

func (r searchResult) candidate(searched string) (*slotsv1.TitleCandidate, bool) {
	kind, known := resultKind(searched, r)
	if r.ID == 0 || !known {
		return nil, false
	}
	title := firstNonEmpty(r.Title, r.Name)
	orig := firstNonEmpty(r.OriginalTitle, r.OriginalName)
	c := &slotsv1.TitleCandidate{
		Ref:           "tmdb:" + strconv.Itoa(r.ID),
		Kind:        kind,
		Title:        title,
		OriginalTitle: orig,
		Year:          yearOf(firstNonEmpty(r.ReleaseDate, r.FirstAirDate)),
	}
	if r.IMDbID != "" {
		c.ExternalIds = append(c.ExternalIds, &slotsv1.ExternalId{Namespace: "imdb", Value: r.IMDbID})
	}
	return c, title != ""
}

// GetMetadata resolves any asserted external ID to the full record. Native
// refs go straight to the details endpoint; foreign ones resolve through
// the /find bridge first (§1.27: mapping is adapter work behind the
// contract). A tmdb ref carries no kind, so the movie endpoint is tried
// first and series second — IDs are effectively disjoint and the fallback
// costs one extra round trip at most.
func (s *Slot) GetMetadata(ctx context.Context, req *slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
	ns, val, ok := strings.Cut(req.GetRef(), ":")
	if !ok || ns == "" || val == "" {
		return nil, fmt.Errorf("tmdb: ref %q is not namespace:value", req.GetRef())
	}
	var (
		id    string
		media string
	)
	switch ns {
	case "tmdb":
		id = val
		m, err := s.resolveNative(ctx, val)
		if err != nil {
			return nil, err
		}
		media = m
	case "imdb":
		id, media = "", ""
		if err := s.findBy(ctx, val, "imdb_id", &id, &media); err != nil {
			return nil, err
		}
	case "tvdb":
		id, media = "", ""
		if err := s.findBy(ctx, val, "tvdb_id", &id, &media); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("tmdb: unknown namespace %q in ref %q", ns, req.GetRef())
	}

	d, err := s.details(ctx, media, id)
	if err != nil {
		return nil, err
	}
	return d.response(media), nil
}

// details fetches the full record in one round trip via append_to_response
// (§1.27: no N+1). The append list differs per media type.
func (s *Slot) details(ctx context.Context, media, id string) (*detail, error) {
	append := "external_ids,credits,content_ratings"
	params := url.Values{"append_to_response": {append}, "language": {language}}
	if media == "movie" {
		params.Set("append_to_response", "external_ids,credits,release_dates")
	}
	var d detail
	if err := s.get(ctx, "/"+media+"/"+url.PathEscape(id), params, &d); err != nil {
		return nil, fmt.Errorf("tmdb: details %s/%s: %w", media, id, err)
	}
	return &d, nil
}

func (s *Slot) resolveNative(ctx context.Context, id string) (string, error) {
	for _, media := range []string{"movie", "tv"} {
		exists := struct {
			Success bool `json:"success"`
		}{}
		err := s.get(ctx, "/"+media+"/"+url.PathEscape(id), url.Values{}, &exists)
		if err == nil {
			return media, nil
		}
		if !isNotFound(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("tmdb: no movie or series with id %q", id)
}

func (s *Slot) findBy(ctx context.Context, val, source string, id, media *string) error {
	params := url.Values{"external_source": {source}}
	var page struct {
		MovieResults []struct {
			ID int `json:"id"`
		} `json:"movie_results"`
		TVResults []struct {
			ID int `json:"id"`
		} `json:"tv_results"`
	}
	if err := s.get(ctx, "/find/"+url.PathEscape(val), params, &page); err != nil {
		return err
	}
	switch {
	case len(page.MovieResults) > 0:
		*id, *media = strconv.Itoa(page.MovieResults[0].ID), "movie"
	case len(page.TVResults) > 0:
		*id, *media = strconv.Itoa(page.TVResults[0].ID), "tv"
	default:
		return fmt.Errorf("tmdb: %s %q resolved to no title", source, val)
	}
	return nil
}

type detail struct {
	ID              int    `json:"id"`
	Title             string `json:"title,omitempty"`
	Name               string `json:"name,omitempty"`
	OriginalTitle string `json:"original_title,omitempty"`
	OriginalName  string `json:"original_name,omitempty"`
	Overview        string `json:"overview,omitempty"`
	ReleaseDate     string `json:"release_date,omitempty"`
	FirstAirDate  string `json:"first_air_date,omitempty"`
	VoteAverage     float64 `json:"vote_average"`
	PosterPath      string `json:"poster_path,omitempty"`
	OriginalLanguage string  `json:"original_language,omitempty"`
	Runtime            int     `json:"runtime,omitempty"`
	NumberOfSeasons  int     `json:"number_of_seasons,omitempty"`
	NumberOfEpisodes int     `json:"number_of_episodes,omitempty"`

	Credits struct {
		Cast []struct {
			Name  string `json:"name"`
			Order int    `json:"order"`
		} `json:"cast"`
		Crew []struct {
			Name   string `json:"name"`
			Job    string `json:"job"`
		} `json:"crew"`
	} `json:"credits"`

	ExternalIDs struct {
		IMDbID      string  `json:"imdb_id"`
		WikidataID  string  `json:"wikidata_id"`
		TVDBID      json.RawMessage `json:"tvdb_id"`
	} `json:"external_ids"`

	ReleaseDates struct {
		Results []struct {
			Country       string `json:"iso_3166_1"`
			ReleaseDates []struct {
				Certification string `json:"certification"`
			} `json:"release_dates"`
		} `json:"results"`
	} `json:"release_dates"`

	ContentRatings struct {
		Results []struct {
			Country string `json:"iso_3166_1"`
			Rating  string `json:"rating"`
		} `json:"results"`
	} `json:"content_ratings"`
}

func (d *detail) response(media string) *slotsv1.GetMetadataResponse {
	md := &corev1.TitleMetadata{
		Title:            firstNonEmpty(d.Title, d.Name),
		Year:            yearOf(firstNonEmpty(d.ReleaseDate, d.FirstAirDate)),
		Description:    d.Overview,
		Rating:          float32(d.VoteAverage),
		PosterUrl:      posterURL(d.PosterPath),
		OriginalLanguage: d.OriginalLanguage,
	}
	if media == "movie" {
		if d.Runtime > 0 {
			md.KindSpecific = &corev1.TitleMetadata_Movie{
				Movie: &corev1.MovieSpecific{RuntimeMinutes: uint32(d.Runtime)},
			}
		}
		md.ContentRating = d.movieCertification()
	} else {
		md.KindSpecific = &corev1.TitleMetadata_Series{
			Series: &corev1.SeriesSpecific{
				TotalSeasons:  uint32(d.NumberOfSeasons),
				TotalEpisodes: uint32(d.NumberOfEpisodes),
			},
		}
		md.ContentRating = d.tvRating()
	}
	for _, c := range d.Credits.Cast {
		if len(md.Cast) == maxCast {
			break
		}
		if c.Name != "" {
			md.Cast = append(md.Cast, c.Name)
		}
	}
	for _, c := range d.Credits.Crew {
		if c.Job == "Director" && c.Name != "" && !contains(md.Directors, c.Name) {
			md.Directors = append(md.Directors, c.Name)
		}
	}

	resp := &slotsv1.GetMetadataResponse{Metadata: md}
	resp.ExternalIds = append(resp.ExternalIds, &slotsv1.ExternalId{Namespace: "tmdb", Value: strconv.Itoa(d.ID)})
	if d.ExternalIDs.IMDbID != "" {
		resp.ExternalIds = append(resp.ExternalIds, &slotsv1.ExternalId{Namespace: "imdb", Value: d.ExternalIDs.IMDbID})
	}
	if d.ExternalIDs.WikidataID != "" {
		resp.ExternalIds = append(resp.ExternalIds, &slotsv1.ExternalId{Namespace: "wikidata", Value: d.ExternalIDs.WikidataID})
	}
	if tvdb := d.tvdbID(); tvdb != "" {
		resp.ExternalIds = append(resp.ExternalIds, &slotsv1.ExternalId{Namespace: "tvdb", Value: tvdb})
	}
	return resp
}

func (d *detail) movieCertification() string {
	for _, r := range d.ReleaseDates.Results {
		if r.Country == "US" {
			for _, rd := range r.ReleaseDates {
				if rd.Certification != "" {
					return rd.Certification
				}
			}
		}
	}
	return ""
}

func (d *detail) tvRating() string {
	for _, r := range d.ContentRatings.Results {
		if r.Country == "US" {
			return r.Rating
		}
	}
	return ""
}

func (d *detail) tvdbID() string {
	raw := d.ExternalIDs.TVDBID
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return strconv.FormatInt(n, 10)
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	return ""
}

// get performs one paced GET with the bearer credential attached, backing
// off once on a 429 per Retry-After (§1.27) before surfacing the failure.
func (s *Slot) get(ctx context.Context, path string, params url.Values, into any) error {
	body, err := s.fetch(ctx, path, params)
	if isRateLimited(err) {
		time.Sleep(backoffFor(err))
		body, err = s.fetch(ctx, path, params)
	}
	if err != nil {
		return err
	}
	defer closeBody(body)
	return json.NewDecoder(body).Decode(into)
}

func (s *Slot) fetch(ctx context.Context, path string, params url.Values) (io.ReadCloser, error) {
	select {
	case <-s.pace:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	u := s.base + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/json")
	resp, err := s.hc.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil
	case http.StatusTooManyRequests:
		closeBody(resp.Body)
		return nil, &rateLimited{retryAfter: retryAfterOf(resp.Header.Get("Retry-After"))}
	default:
		closeBody(resp.Body)
		return nil, statusError(resp.StatusCode)
	}
}

type rateLimited struct{ retryAfter time.Duration }

func (e *rateLimited) Error() string { return "tmdb: rate limited (429)" }

func isRateLimited(err error) bool {
	_, ok := err.(*rateLimited)
	return ok
}

func backoffFor(err error) time.Duration {
	if rl, ok := err.(*rateLimited); ok && rl.retryAfter > 0 && rl.retryAfter <= maxBackoff {
		return rl.retryAfter
	}
	return time.Second
}

func retryAfterOf(header string) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

func isNotFound(err error) bool {
	se, ok := err.(statusErr)
	return ok && se.code == http.StatusNotFound
}

type statusErr struct{ code int }

func (e statusErr) Error() string { return fmt.Sprintf("tmdb: HTTP %d", e.code) }

func statusError(code int) error { return statusErr{code: code} }

func closeBody(b io.ReadCloser) { _ = b.Close() }

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func yearOf(date string) uint32 {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.ParseUint(date[:4], 10, 32)
	if err != nil {
		return 0
	}
	return uint32(y)
}

func posterURL(path string) string {
	if path == "" {
		return ""
	}
	return imageBase + posterSize + path
}
