package delivery

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nem-git/abcmovies/core/drm"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Opener produces a track's clear bytes for delivery (PLAN.md §6.6). It is
// the engine's single byte-opening choke point: pull sinks (relay) consume
// Fetch, push sinks (disk) consume Open. Both share the same clear-bytes
// flow — fetch the provider source, acquire keys through the content-key
// cache on miss, decrypt, and fail-fast re-license on decrypt failure.
//
// The opener is engine-built (it owns the DRM client and content-key cache)
// and handed to sinks, so sinks never touch DRM directly.
type Opener struct {
	drm    DRMClient
	cache  ContentKeyCache
	http   *http.Client
	logger *slog.Logger
	now    func() time.Time
}

// NewOpener builds an Opener from the engine's DRM client and content-key
// cache. A nil DRM client or cache is valid: non-DRM tracks are unaffected,
// and DRM-encrypted tracks are refused (no supported system).
func NewOpener(drm DRMClient, cache ContentKeyCache, logger *slog.Logger) *Opener {
	if logger == nil {
		logger = slog.Default()
	}
	return &Opener{
		drm:    drm,
		cache:  cache,
		http:   &http.Client{Timeout: 30 * time.Second},
		logger: logger,
		now:    time.Now,
	}
}

// Fetch returns a lazy pull source for a track (relay tickets). The
// returned FetchFunc captures the track's clear-bytes flow so a relay pull
// serves clear bytes on demand.
func (o *Opener) Fetch(s *Session, track *corev1.Track) FetchFunc {
	return func(ctx context.Context, rangeHeader string) (FetchResult, error) {
		return o.fetchClear(ctx, s, track, rangeHeader)
	}
}

// Open returns a reader over the track's clear bytes for push delivery
// (e.g. a disk sink). It is the eager counterpart to Fetch.
func (o *Opener) Open(ctx context.Context, s *Session, track *corev1.Track) (io.ReadCloser, error) {
	res, err := o.fetchClear(ctx, s, track, "")
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

// fetchClear is the shared clear-bytes flow. It fetches the provider source
// and, for DRM-encrypted tracks, runs the cache → acquire → decrypt path
// with fail-fast re-license. A non-DRM track is a plain passthrough.
func (o *Opener) fetchClear(ctx context.Context, s *Session, track *corev1.Track, rangeHeader string) (FetchResult, error) {
	dl := track.GetDelivery()
	if dl == nil {
		return FetchResult{}, fmt.Errorf("opener: track %q has no delivery", track.GetId())
	}
	if len(dl.GetLocations()) == 0 {
		return FetchResult{}, fmt.Errorf("opener: track %q has no locations", track.GetId())
	}

	// No DRM — plain passthrough.
	if len(dl.GetDrm()) == 0 {
		return o.fetchPlain(ctx, track, rangeHeader)
	}

	providerID := s.Context.GetProvider()
	contentID := s.Context.GetNativeId()

	for _, drmEntry := range dl.GetDrm() {
		system, ok := slotsv1.DrmSystem_value[drmEntry.GetSystem()]
		if !ok {
			continue
		}
		sys := slotsv1.DrmSystem(system)
		if !o.drm.Supports(sys) {
			continue
		}

		keys, err := o.getKeys(ctx, providerID, contentID, sys, drmEntry, drmEntry.GetLicenseUrl(), dl.GetAuthContext())
		if err != nil {
			return FetchResult{}, err
		}

		clear, err := o.decryptWithRetry(ctx, providerID, contentID, sys, track, keys, drmEntry, drmEntry.GetLicenseUrl(), dl.GetAuthContext())
		if err != nil {
			return FetchResult{}, err
		}

		return o.serveClear(clear, rangeHeader), nil
	}

	return FetchResult{}, fmt.Errorf("opener: track %q has DRM but no supported system", track.GetId())
}

// fetchPlain is the non-DRM passthrough: fetch the provider source
// directly, forwarding any range request.
func (o *Opener) fetchPlain(ctx context.Context, track *corev1.Track, rangeHeader string) (FetchResult, error) {
	dl := track.GetDelivery()
	location := dl.GetLocations()[0]
	auth := dl.GetAuthContext()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return FetchResult{}, err
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := o.http.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("opener: provider fetch: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		_ = resp.Body.Close()
		return FetchResult{}, fmt.Errorf("opener: provider returned %s", resp.Status)
	}
	return FetchResult{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: resp.Body}, nil
}

// getKeys returns cached keys for the content and system, or acquires them
// on cache miss. Acquired keys are stored in the cache for the next pull.
func (o *Opener) getKeys(ctx context.Context, providerID, contentID string, system slotsv1.DrmSystem, drmEntry *corev1.TrackDrm, licenseURL, authContext string) ([]*slotsv1.ContentKey, error) {
	keys, err := o.cache.GetKeys(ctx, providerID, contentID, system)
	if err != nil {
		o.logger.Warn("opener: cache read failed", "err", err, "provider", providerID, "content", contentID, "system", system)
	}
	if len(keys) > 0 {
		return keys, nil
	}

	keys, err = o.drm.AcquireKeys(ctx, providerID, contentID, system, drmEntry.GetInitData(), drmEntry.GetKeyIds(), licenseURL, authContext)
	if err != nil {
		return nil, fmt.Errorf("opener: DRM could not be decrypted (acquire keys: %v)", err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("opener: DRM could not be decrypted (no keys acquired)")
	}

	if err := o.cache.PutKeys(ctx, providerID, contentID, system, keys); err != nil {
		o.logger.Warn("opener: cache write failed", "err", err)
	}
	return keys, nil
}

// decryptWithRetry decrypts the track, and on failure drops the cached
// keys and re-licenses once (fail-fast re-license — PLAN.md §6.6). If the
// re-license also fails, the error is neutral: "DRM could not be decrypted."
func (o *Opener) decryptWithRetry(ctx context.Context, providerID, contentID string, system slotsv1.DrmSystem, track *corev1.Track, keys []*slotsv1.ContentKey, drmEntry *corev1.TrackDrm, licenseURL, authContext string) ([]byte, error) {
	// Fetch the encrypted source inline (M8: whole-content in memory).
	encResult, err := o.fetchPlain(ctx, track, "")
	if err != nil {
		return nil, fmt.Errorf("opener: fetch encrypted source: %w", err)
	}
	defer func() { _ = encResult.Body.Close() }()
	encBytes, err := io.ReadAll(encResult.Body)
	if err != nil {
		return nil, fmt.Errorf("opener: read encrypted source: %w", err)
	}

	clear, err := o.drm.Decrypt(ctx, drm.Source{InlineBytes: encBytes}, keys)
	if err == nil {
		return clear, nil
	}

	// Fail-fast: drop the bad keys and re-license once.
	_ = o.cache.DropKeys(ctx, providerID, contentID, system)
	o.logger.Warn("opener: decrypt failed, re-licensing once", "provider", providerID, "content", contentID, "system", system)

	keys, err = o.drm.AcquireKeys(ctx, providerID, contentID, system, drmEntry.GetInitData(), drmEntry.GetKeyIds(), licenseURL, authContext)
	if err != nil {
		return nil, fmt.Errorf("opener: DRM could not be decrypted (re-acquire keys: %v)", err)
	}
	if err := o.cache.PutKeys(ctx, providerID, contentID, system, keys); err != nil {
		o.logger.Warn("opener: cache write failed after re-license", "err", err)
	}

	clear, err = o.drm.Decrypt(ctx, drm.Source{InlineBytes: encBytes}, keys)
	if err != nil {
		_ = o.cache.DropKeys(ctx, providerID, contentID, system)
		return nil, fmt.Errorf("opener: DRM could not be decrypted")
	}
	return clear, nil
}

// serveClear builds a FetchResult over the clear bytes. A range request
// slices the decrypted buffer; otherwise the full clear content is served.
func (o *Opener) serveClear(clear []byte, rangeHeader string) FetchResult {
	status := http.StatusOK
	if rangeHeader != "" {
		start, end, ok := parseRange(rangeHeader, len(clear))
		if ok && start < end && start < len(clear) {
			clear = clear[start:min(end, len(clear))]
			status = http.StatusPartialContent
		}
	}
	return FetchResult{
		StatusCode: status,
		Header: http.Header{
			"Content-Length": []string{strconv.Itoa(len(clear))},
		},
		Body: io.NopCloser(bytes.NewReader(clear)),
	}
}

// parseRange parses a single-range HTTP Range header ("bytes=start-end") and
// returns the zero-based [start, end) slice bounds for a buffer of size n.
// ok=false for malformed or unsatisfiable ranges.
func parseRange(header string, n int) (start, end int, ok bool) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(header, "bytes=")
	if strings.Contains(spec, ",") {
		return 0, 0, false // multi-range not supported
	}
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])
	if startStr == "" {
		// Suffix range: -N means last N bytes.
		if endStr == "" {
			return 0, 0, false
		}
		suffix, err := strconv.Atoi(endStr)
		if err != nil || suffix <= 0 || n == 0 {
			return 0, 0, false
		}
		if suffix > n {
			suffix = n
		}
		return n - suffix, n, true
	}
	var err error
	start, err = strconv.Atoi(startStr)
	if err != nil || start < 0 || start >= n {
		return 0, 0, false
	}
	if endStr == "" {
		return start, n, true
	}
	end, err = strconv.Atoi(endStr)
	if err != nil || end < start {
		return 0, 0, false
	}
	if end >= n {
		end = n - 1
	}
	return start, end + 1, true
}
