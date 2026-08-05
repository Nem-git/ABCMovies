package proxy

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Eyevinn/dash-mpd/mpd"
	"github.com/nem-git/abcmovies/internal/proxy/parser"
	"github.com/nem-git/abcmovies/internal/stream"
)

// DASHStrategy handles DASH manifest rewriting with RESTful URL scheme.
type DASHStrategy struct {
	deps StrategyDeps
}

// NewDASHStrategy creates a DASH strategy with the given dependencies.
func NewDASHStrategy(deps StrategyDeps) *DASHStrategy {
	return &DASHStrategy{deps: deps}
}

func (s *DASHStrategy) ServeManifest(ctx context.Context, w io.Writer, locator stream.Locator, meta *StreamMeta) (string, error) {
	body, _, err := s.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return "", err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	p := parser.DASHParser{}
	m, err := p.Read(data)
	if err != nil {
		return "", fmt.Errorf("parse DASH MPD: %w", err)
	}

	upstreamBaseURL := ResolveBaseURL(locator.URL)

	// Rewrite SegmentTemplate URLs per Representation
	for periodIdx, period := range m.Periods {
		for asIdx, as := range period.AdaptationSets {
			// When SegmentTemplate is at AdaptationSet or Period level, all reps
			// share the same pointer. We must NOT mutate it — instead, snapshot
			// the raw values and create per-representation copies.
			var sharedRawMedia, sharedRawInit string
			if as.SegmentTemplate != nil {
				sharedRawMedia = as.SegmentTemplate.Media
				sharedRawInit = as.SegmentTemplate.Initialization
			} else if period.SegmentTemplate != nil {
				sharedRawMedia = period.SegmentTemplate.Media
				sharedRawInit = period.SegmentTemplate.Initialization
			}
			sharedTemplate := parser.EffectiveSegmentTemplate(as.Representations[0])
			hasSharedTemplate := sharedTemplate != nil && (sharedRawMedia != "" || sharedRawInit != "")

			for _, rep := range as.Representations {
				st := parser.EffectiveSegmentTemplate(rep)
				if st == nil {
					continue
				}

				repID := rep.Id
				bandwidth := strconv.FormatUint(uint64(rep.Bandwidth), 10)
				periodPrefix := path.Join(meta.ProxyBaseURL, "periods", strconv.Itoa(periodIdx), "adaptation-sets", strconv.Itoa(asIdx), "representations", repID)

				// Use shared snapshot if template is inherited from AS/Period
				rawMedia := sharedRawMedia
				if rawMedia == "" {
					rawMedia = st.Media
				}
				rawInit := sharedRawInit
				if rawInit == "" {
					rawInit = st.Initialization
				}

				// Compute original upstream templates from the raw values
				var originalMediaTemplate, originalInitTemplate string
				if rawMedia != "" {
					originalMediaTemplate = parser.ResolveURL(upstreamBaseURL, rawMedia)
				}
				if rawInit != "" {
					originalInitTemplate = parser.ResolveURL(upstreamBaseURL, rawInit)
				}

				// Build rewritten templates
				var newMedia, newInit string
				if rawMedia != "" {
					newMedia = path.Join(periodPrefix, "segments", path.Base(s.rewriteTemplate(rawMedia, repID, bandwidth)))
				}
				if rawInit != "" {
					newInit = path.Join(periodPrefix, "segments", "init")
				}

				if hasSharedTemplate {
					// Create a per-representation SegmentTemplate to avoid mutating the shared one
					repST := &mpd.SegmentTemplateType{
						Media:                   newMedia,
						Initialization:          newInit,
						MultipleSegmentBaseType: st.MultipleSegmentBaseType,
					}
					rep.SegmentTemplate = repST
				} else {
					// Rep-level template, safe to mutate directly
					if newMedia != "" {
						st.Media = newMedia
					}
					if newInit != "" {
						st.Initialization = newInit
					}
				}

				// Store state for segment resolution (original upstream templates)
				stateKey := dashStateKey(meta.ProviderTag, meta.ContentKey, periodIdx, asIdx, repID)
				stateMeta := *meta
				stateMeta.UpstreamMediaTemplate = originalMediaTemplate
				stateMeta.UpstreamInitTemplate = originalInitTemplate
				stateMeta.UpstreamRepID = repID
				stateMeta.UpstreamBandwidth = bandwidth
				stateMeta.ExpiresAt = time.Now().Add(5 * time.Minute)
				s.deps.State.Put(ctx, stateKey, stateMeta)
			}

			// Clear shared AS-level template since per-rep copies were created
			if hasSharedTemplate {
				as.SegmentTemplate = nil
			}
		}
	}

	// Rewrite BaseURL for SegmentBase representations
	for _, period := range m.Periods {
		for _, as := range period.AdaptationSets {
			for _, rep := range as.Representations {
				if rep.SegmentBase != nil && len(rep.BaseURLs) > 0 {
					baseURLVal := string(rep.BaseURLs[0].Value)
					abs := parser.ResolveURL(upstreamBaseURL, baseURLVal)
					u, err := parseURL(abs)
					if err == nil {
						dir := filepath.Dir(u.Path)
						u.Path = path.Join(dir, filepath.Base(u.Path))
						rep.BaseURLs[0].Value = mpd.AnyURI(u.String())
					}
				}
			}
		}
	}

	rewritten, err := p.Write(m)
	if err != nil {
		return "", err
	}
	w.Write(rewritten)

	return upstreamBaseURL, nil
}

// rewriteTemplate prepares a SegmentTemplate URL for the proxy response.
// $RepresentationID$ and $Bandwidth$ are resolved to their literal values so the
// rewritten template is deterministic. $Number$ and $Time$ are left as placeholders —
// the player resolves them per ISO/IEC 23009-1.
func (s *DASHStrategy) rewriteTemplate(template, repID, bandwidth string) string {
	result := template
	result = strings.ReplaceAll(result, "$RepresentationID$", repID)
	result = strings.ReplaceAll(result, "$Bandwidth$", bandwidth)
	return result
}

// stripSegmentPlaceholders removes $Number...$ and $Time...$ tokens (including any
// format suffix such as %05d) from a template URL. Used when reconstructing init
// URLs, which never carry segment-specific placeholders.
func stripSegmentPlaceholders(s string) string {
	for _, marker := range []string{"$Number", "$Time"} {
		if idx := strings.Index(s, marker); idx != -1 {
			rest := s[idx+len(marker):]
			if end := strings.Index(rest, "$"); end != -1 {
				s = s[:idx] + rest[end+1:]
			}
		}
	}
	return s
}

// ServeSegment fetches a DASH segment by reconstructing the upstream URL from the template.
func (s *DASHStrategy) ServeSegment(ctx context.Context, w io.Writer, locator stream.Locator, segmentPath string) error {
	body, _, err := s.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return err
	}
	defer body.Close()

	io.Copy(w, body)
	return nil
}

// ServeInitSegment fetches a DASH init segment by reconstructing the upstream URL from the template.
func (s *DASHStrategy) ServeInitSegment(ctx context.Context, w io.Writer, locator stream.Locator) error {
	body, _, err := s.deps.Fetcher.Fetch(ctx, locator.URL, locator.Headers, locator.Query)
	if err != nil {
		return err
	}
	defer body.Close()

	io.Copy(w, body)
	return nil
}

// dashStateKey builds a state key for a DASH representation.
func dashStateKey(tag, contentKey string, periodIdx, asIdx int, repID string) string {
	return tag + ":" + contentKey + ":dash:" + strconv.Itoa(periodIdx) + ":" + strconv.Itoa(asIdx) + ":" + repID
}

// DASHStateKey returns the state key for a DASH representation entry.
func DASHStateKey(tag, contentKey string, periodIdx, asIdx int, repID string) string {
	return dashStateKey(tag, contentKey, periodIdx, asIdx, repID)
}

// parseURL is a helper that wraps url.Parse.
func parseURL(rawURL string) (*url.URL, error) {
	return url.Parse(rawURL)
}
