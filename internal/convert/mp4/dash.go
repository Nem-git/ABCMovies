package mp4

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Eyevinn/dash-mpd/mpd"

	"github.com/nem-git/abcmovies/internal/proxy/parser"
	"github.com/nem-git/abcmovies/internal/stream"
)

var (
	numberFmtRe = regexp.MustCompile(`\$Number%([0-9]+)d\$`)
	numberRe    = regexp.MustCompile(`\$Number\$`)
	timeFmtRe   = regexp.MustCompile(`\$Time%([0-9]+)d\$`)
	timeRe      = regexp.MustCompile(`\$Time\$`)
)

// convertDASH converts a DASH MPD to a single fragmented MP4.
func (c *Converter) convertDASH(ctx context.Context, src *stream.Locator, w io.Writer) error {
	data, err := c.fetch(ctx, src, src.URL)
	if err != nil {
		return fmt.Errorf("convert: fetching DASH manifest: %w", err)
	}
	m, err := (&parser.DASHParser{}).Read(data)
	if err != nil {
		return fmt.Errorf("convert: parsing DASH manifest: %w", err)
	}

	mpdBase := src.URL
	if len(m.BaseURL) > 0 && m.BaseURL[0].Value != "" {
		mpdBase = parser.ResolveURL(src.URL, string(m.BaseURL[0].Value))
	}

	var totalDur float64
	if m.MediaPresentationDuration != nil {
		totalDur = m.MediaPresentationDuration.Seconds()
	}

	for _, period := range m.Periods {
		periodDur := totalDur
		if period.Duration != nil {
			periodDur = period.Duration.Seconds()
		}
		tracks, err := c.dashTracks(mpdBase, period, periodDur)
		if err != nil {
			return err
		}
		if len(tracks) == 0 {
			continue
		}
		if err := c.writeFMP4(ctx, src, tracks, w); err != nil {
			return err
		}
	}
	if len(m.Periods) == 0 {
		return fmt.Errorf("convert: DASH manifest has no periods")
	}
	return nil
}

// dashTracks selects the highest-bandwidth representation from each media
// adaptation set in the period and resolves its init + segment URLs.
func (c *Converter) dashTracks(base string, period *mpd.Period, periodDur float64) ([]fmp4Track, error) {
	var tracks []fmp4Track
	for _, as := range period.AdaptationSets {
		if as == nil || isTextAdaptationSet(as) {
			continue
		}
		rep := highestBandwidthRepresentation(as)
		if rep == nil {
			continue
		}
		mt := adaptationSetMediaType(as)
		t := fmp4Track{label: "DASH " + mt + " representation " + rep.Id}
		asBase := base
		if len(period.BaseURLs) > 0 && period.BaseURLs[0].Value != "" {
			asBase = parser.ResolveURL(asBase, string(period.BaseURLs[0].Value))
		}
		if len(as.BaseURLs) > 0 && as.BaseURLs[0].Value != "" {
			asBase = parser.ResolveURL(asBase, string(as.BaseURLs[0].Value))
		}
		if len(rep.BaseURLs) > 0 && rep.BaseURLs[0].Value != "" {
			asBase = parser.ResolveURL(asBase, string(rep.BaseURLs[0].Value))
		}

		initURL, segURLs, err := c.representationSegments(asBase, as, rep, periodDur)
		if err != nil {
			return nil, err
		}
		if len(segURLs) == 0 {
			continue
		}
		t.initURL = initURL
		t.segmentURLs = segURLs
		tracks = append(tracks, t)
	}
	// Video first so it gets the lowest output track ID.
	sort.SliceStable(tracks, func(i, j int) bool {
		return trackMediaType(tracks[i]) == "video" && trackMediaType(tracks[j]) != "video"
	})
	return tracks, nil
}

// trackMediaType reports the media type of a track from its label.
func trackMediaType(t fmp4Track) string {
	switch {
	case strings.HasPrefix(t.label, "DASH audio"), strings.HasPrefix(t.label, "HLS audio"):
		return "audio"
	default:
		return "video"
	}
}

// adaptationSetMediaType classifies an adaptation set as video, audio or text.
func adaptationSetMediaType(as *mpd.AdaptationSetType) string {
	ct := strings.ToLower(string(as.ContentType))
	if ct == "audio" || strings.HasSuffix(ct, "audio") {
		return "audio"
	}
	if ct == "text" || ct == "subtitles" || strings.HasSuffix(ct, "caption") {
		return "text"
	}
	mime := as.GetMimeType()
	switch {
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	}
	return "video"
}

// isTextAdaptationSet reports whether an adaptation set carries subtitle/text
// content that should not be muxed into the output.
func isTextAdaptationSet(as *mpd.AdaptationSetType) bool {
	ct := strings.ToLower(string(as.ContentType))
	if ct == "text" || ct == "subtitles" || strings.HasSuffix(ct, "caption") {
		return true
	}
	mime := as.GetMimeType()
	return strings.HasPrefix(mime, "text/") ||
		strings.Contains(mime, "stpp") ||
		strings.Contains(mime, "wvtt") ||
		strings.Contains(mime, "ttml")
}

// highestBandwidthRepresentation returns the representation with the largest
// bandwidth in an adaptation set.
func highestBandwidthRepresentation(as *mpd.AdaptationSetType) *mpd.RepresentationType {
	var best *mpd.RepresentationType
	for _, rep := range as.Representations {
		if rep == nil {
			continue
		}
		if best == nil || rep.Bandwidth > best.Bandwidth {
			best = rep
		}
	}
	return best
}

// representationSegments resolves the init segment URL and the ordered list of
// media segment URLs for a representation using SegmentTemplate or SegmentList
// addressing.
func (c *Converter) representationSegments(base string, as *mpd.AdaptationSetType, rep *mpd.RepresentationType, periodDur float64) (string, []string, error) {
	if st := parser.EffectiveSegmentTemplate(rep); st != nil {
		return c.segmentTemplateURLs(base, as, rep, st, periodDur)
	}
	if sl := effectiveSegmentList(as, rep); sl != nil {
		return c.segmentListURLs(base, sl)
	}
	if effectiveSegmentBase(as, rep) != nil {
		return "", nil, fmt.Errorf("convert: DASH SegmentBase addressing is not supported")
	}
	return "", nil, fmt.Errorf("convert: unsupported DASH segment addressing for representation %s", rep.Id)
}

// effectiveSegmentList walks the hierarchy to find the effective SegmentList.
func effectiveSegmentList(as *mpd.AdaptationSetType, rep *mpd.RepresentationType) *mpd.SegmentListType {
	if rep != nil && rep.SegmentList != nil {
		return rep.SegmentList
	}
	if as != nil && as.SegmentList != nil {
		return as.SegmentList
	}
	if as != nil && as.Parent() != nil {
		return as.Parent().SegmentList
	}
	return nil
}

// effectiveSegmentBase walks the hierarchy to find the effective SegmentBase.
func effectiveSegmentBase(as *mpd.AdaptationSetType, rep *mpd.RepresentationType) *mpd.SegmentBaseType {
	if rep != nil && rep.SegmentBase != nil {
		return rep.SegmentBase
	}
	if as != nil && as.SegmentBase != nil {
		return as.SegmentBase
	}
	if as != nil && as.Parent() != nil {
		return as.Parent().SegmentBase
	}
	return nil
}

// segmentTemplateURLs resolves init + media segments from a SegmentTemplate.
func (c *Converter) segmentTemplateURLs(base string, as *mpd.AdaptationSetType, rep *mpd.RepresentationType, st *mpd.SegmentTemplateType, periodDur float64) (string, []string, error) {
	if st.Media == "" {
		return "", nil, fmt.Errorf("convert: SegmentTemplate has no media template")
	}
	initURL := ""
	if st.Initialization != "" {
		initURL = parser.ResolveURL(base, expandTemplate(st.Initialization, rep, 0, 0))
	}

	var start uint32 = 1
	if st.StartNumber != nil {
		start = *st.StartNumber
	}

	var urls []string
	switch {
	case st.SegmentTimeline != nil:
		n := start
		var currentTime uint64
		for _, s := range st.SegmentTimeline.S {
			if s.R == -1 {
				return "", nil, fmt.Errorf("convert: open-ended SegmentTimeline not supported")
			}
			if s.T != nil {
				currentTime = *s.T
			}
			for k := 0; k <= s.R; k++ {
				urls = append(urls, parser.ResolveURL(base, expandTemplate(st.Media, rep, n, currentTime)))
				currentTime += s.D
				n++
			}
		}
	case st.Duration != nil && *st.Duration > 0:
		segDur := float64(*st.Duration) / float64(st.GetTimescale())
		var count int
		if st.EndNumber != nil {
			count = int(*st.EndNumber) - int(start) + 1
		} else if periodDur > 0 && segDur > 0 {
			count = int(math.Ceil(periodDur / segDur))
		} else {
			return "", nil, fmt.Errorf("convert: cannot determine DASH segment count (no timeline, duration or endNumber)")
		}
		if count < 0 {
			count = 0
		}
		for i := 0; i < count; i++ {
			urls = append(urls, parser.ResolveURL(base, expandTemplate(st.Media, rep, start+uint32(i), 0)))
		}
	default:
		return "", nil, fmt.Errorf("convert: SegmentTemplate has no duration or timeline")
	}

	if initURL == "" {
		return "", nil, fmt.Errorf("convert: no initialization segment template for representation %s", rep.Id)
	}
	return initURL, urls, nil
}

// segmentListURLs resolves init + media segments from a SegmentList.
func (c *Converter) segmentListURLs(base string, sl *mpd.SegmentListType) (string, []string, error) {
	initURL := ""
	if sl.Initialization != nil {
		initURL = parser.ResolveURL(base, string(sl.Initialization.SourceURL))
	}
	var urls []string
	for _, su := range sl.SegmentURL {
		if su == nil || su.Media == "" {
			continue
		}
		urls = append(urls, parser.ResolveURL(base, string(su.Media)))
	}
	if initURL == "" {
		return "", nil, fmt.Errorf("convert: SegmentList has no initialization")
	}
	if len(urls) == 0 {
		return "", nil, errors.New("convert: SegmentList has no segments")
	}
	return initURL, urls, nil
}

// expandTemplate replaces $RepresentationID$, $Bandwidth$, $Number[%0Nd]$ and
// $Time[%0Nd]$ placeholders in a DASH template.
func expandTemplate(tpl string, rep *mpd.RepresentationType, n uint32, t uint64) string {
	r := strings.ReplaceAll(tpl, "$RepresentationID$", rep.Id)
	r = strings.ReplaceAll(r, "$Bandwidth$", strconv.FormatUint(uint64(rep.Bandwidth), 10))
	if m := numberFmtRe.FindStringSubmatch(r); m != nil {
		width, _ := strconv.Atoi(m[1])
		r = numberFmtRe.ReplaceAllString(r, fmt.Sprintf("%0*d", width, n))
	} else {
		r = numberRe.ReplaceAllString(r, strconv.FormatUint(uint64(n), 10))
	}
	if m := timeFmtRe.FindStringSubmatch(r); m != nil {
		width, _ := strconv.Atoi(m[1])
		r = timeFmtRe.ReplaceAllString(r, fmt.Sprintf("%0*d", width, t))
	} else {
		r = timeRe.ReplaceAllString(r, strconv.FormatUint(t, 10))
	}
	return r
}
