package handler

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/nem-git/abcmovies/internal/oas"
)

type requestContextKey struct{}

// WithRequest injects the incoming *http.Request into the request context so
// that handlers can derive absolute URLs (scheme + host) for the resources
// they return.
func WithRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), requestContextKey{}, r)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requestFrom returns the *http.Request stored by WithRequest, or nil.
func requestFrom(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestContextKey{}).(*http.Request)
	return r
}

// requestOrigin returns the scheme://host the current request was made to,
// honoring X-Forwarded-Proto / X-Forwarded-Host when the server sits behind a
// reverse proxy. Returns "" when no request is available.
func requestOrigin(ctx context.Context) string {
	r := requestFrom(ctx)
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if fwd := firstHeaderValue(r.Header.Get("X-Forwarded-Proto")); fwd != "" {
		scheme = fwd
	}
	if fwd := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); fwd != "" {
		host = fwd
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func firstHeaderValue(v string) string {
	if before, _, ok := strings.Cut(v, ","); ok {
		return strings.TrimSpace(before)
	}
	return strings.TrimSpace(v)
}

// absolutizeURI rewrites a relative URI into an absolute one. The base is the
// configured base_url when present, otherwise the scheme+host of the incoming
// request. Already-absolute and unset URIs are returned unchanged.
func (h *Handler) absolutizeURI(ctx context.Context, u oas.OptURI) oas.OptURI {
	if !u.Set || u.Value.IsAbs() {
		return u
	}
	base := h.baseURL
	if base == "" {
		base = requestOrigin(ctx)
	}
	if base == "" {
		return u
	}
	bu, err := url.Parse(base)
	if err != nil || bu.Scheme == "" || bu.Host == "" {
		return u
	}
	v := u.Value
	v.Scheme = bu.Scheme
	v.Host = bu.Host
	if p := bu.Path; p != "" && p != "/" {
		v.Path = strings.TrimRight(p, "/") + "/" + strings.TrimLeft(v.Path, "/")
	}
	return oas.NewOptURI(v)
}

func (h *Handler) absolutizeMovie(ctx context.Context, m *oas.Movie) {
	m.Poster = h.absolutizeURI(ctx, m.Poster)
	m.Backdrop = h.absolutizeURI(ctx, m.Backdrop)
	m.Trailer = h.absolutizeURI(ctx, m.Trailer)
}

func (h *Handler) absolutizeSeries(ctx context.Context, s *oas.Series) {
	s.Poster = h.absolutizeURI(ctx, s.Poster)
	s.Backdrop = h.absolutizeURI(ctx, s.Backdrop)
	s.Trailer = h.absolutizeURI(ctx, s.Trailer)
}

func (h *Handler) absolutizeSeason(ctx context.Context, s *oas.Season) {
	s.Poster = h.absolutizeURI(ctx, s.Poster)
	s.Backdrop = h.absolutizeURI(ctx, s.Backdrop)
	s.Trailer = h.absolutizeURI(ctx, s.Trailer)
}

func (h *Handler) absolutizeEpisode(ctx context.Context, e *oas.Episode) {
	e.Poster = h.absolutizeURI(ctx, e.Poster)
}

func (h *Handler) absolutizeService(ctx context.Context, s *oas.Service) {
	s.Logo = h.absolutizeURI(ctx, s.Logo)
}

func (h *Handler) absolutizeSubtitle(ctx context.Context, s *oas.Subtitle) {
	s.URL = h.absolutizeURI(ctx, s.URL)
}

func (h *Handler) absolutizeSearchItem(ctx context.Context, item *oas.SearchResultItem) {
	switch item.Resource.Type {
	case oas.MovieSearchResultItemResource:
		h.absolutizeMovie(ctx, &item.Resource.Movie)
	case oas.SeriesSearchResultItemResource:
		h.absolutizeSeries(ctx, &item.Resource.Series)
	case oas.ServiceSearchResultItemResource:
		h.absolutizeService(ctx, &item.Resource.Service)
	}
}
