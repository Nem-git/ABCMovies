package mp4

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nem-git/abcmovies/internal/proxy/parser"
	"github.com/nem-git/abcmovies/internal/stream"
)

// convertHLS converts an HLS stream to a single fragmented MP4.
func (c *Converter) convertHLS(ctx context.Context, src *stream.Locator, w io.Writer) error {
	data, err := c.fetch(ctx, src, src.URL)
	if err != nil {
		return fmt.Errorf("convert: fetching HLS playlist: %w", err)
	}
	master, media, err := (&parser.HLSParser{}).Decode(data)
	if err != nil {
		return fmt.Errorf("convert: parsing HLS playlist: %w", err)
	}
	if master == nil && media == nil {
		return fmt.Errorf("convert: HLS playlist is neither master nor media")
	}

	var videoPlaylist *parser.MediaPlaylist
	var audioPlaylist *parser.MediaPlaylist
	var videoBase, audioBase string

	if master != nil {
		variant := highestBandwidthVariant(master)
		if variant == nil || variant.URI == "" {
			return fmt.Errorf("convert: master playlist has no variants")
		}
		videoBase = parser.ResolveURL(src.URL, variant.URI)
		videoPlaylist, err = c.fetchMediaPlaylist(ctx, src, videoBase)
		if err != nil {
			return fmt.Errorf("convert: fetching variant playlist: %w", err)
		}
		if audioURL := audioRenditionURL(variant); audioURL != "" {
			audioBase = parser.ResolveURL(src.URL, audioURL)
			audioPlaylist, err = c.fetchMediaPlaylist(ctx, src, audioBase)
			if err != nil {
				return fmt.Errorf("convert: fetching audio rendition playlist: %w", err)
			}
		}
	} else {
		videoBase = src.URL
		videoPlaylist = media
	}

	if isTSPlaylist(videoPlaylist) {
		return c.convertTSToMP4(ctx, src, videoPlaylist, audioPlaylist, videoBase, audioBase, w)
	}

	tracks := []fmp4Track{fmp4TrackFromPlaylist(videoBase, videoPlaylist, "video")}
	if audioPlaylist != nil {
		if isTSPlaylist(audioPlaylist) {
			return fmt.Errorf("convert: mixed fMP4 video / TS audio is not supported")
		}
		tracks = append(tracks, fmp4TrackFromPlaylist(audioBase, audioPlaylist, "audio"))
	}
	return c.writeFMP4(ctx, src, tracks, w)
}

// highestBandwidthVariant returns the variant with the largest bandwidth.
func highestBandwidthVariant(master *parser.MasterPlaylist) *parser.Variant {
	var best *parser.Variant
	for _, v := range master.Variants {
		if v == nil {
			continue
		}
		if best == nil || v.Bandwidth > best.Bandwidth {
			best = v
		}
	}
	return best
}

// audioRenditionURL returns the URI of the default audio rendition for a
// variant, if it references an AUDIO group.
func audioRenditionURL(v *parser.Variant) string {
	if v.Audio == "" {
		return ""
	}
	for _, a := range v.Alternatives {
		if a == nil {
			continue
		}
		if a.Type == "AUDIO" && a.GroupId == v.Audio && a.URI != "" {
			return a.URI
		}
	}
	return ""
}

// fetchMediaPlaylist fetches and decodes an HLS media playlist.
func (c *Converter) fetchMediaPlaylist(ctx context.Context, src *stream.Locator, url string) (*parser.MediaPlaylist, error) {
	data, err := c.fetch(ctx, src, url)
	if err != nil {
		return nil, err
	}
	_, media, err := (&parser.HLSParser{}).Decode(data)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, fmt.Errorf("expected a media playlist, got master")
	}
	return media, nil
}

// fmp4TrackFromPlaylist resolves init + segment URLs from an fMP4 media playlist.
func fmp4TrackFromPlaylist(base string, pl *parser.MediaPlaylist, mediaType string) fmp4Track {
	t := fmp4Track{label: "HLS " + mediaType + " track"}
	t.initURL = fmp4InitURL(base, pl)
	for _, seg := range pl.Segments {
		if seg == nil || seg.URI == "" {
			continue
		}
		t.segmentURLs = append(t.segmentURLs, parser.ResolveURL(base, seg.URI))
	}
	return t
}

// fmp4InitURL resolves the EXT-X-MAP init section for an fMP4 media playlist.
func fmp4InitURL(base string, pl *parser.MediaPlaylist) string {
	if len(pl.Segments) > 0 && pl.Segments[0] != nil && pl.Segments[0].Map != nil {
		return parser.ResolveURL(base, pl.Segments[0].Map.URI)
	}
	if pl.Map != nil {
		return parser.ResolveURL(base, pl.Map.URI)
	}
	return ""
}

// isTSPlaylist reports whether a media playlist contains MPEG-TS segments.
func isTSPlaylist(pl *parser.MediaPlaylist) bool {
	if pl.Map != nil {
		return false
	}
	for _, seg := range pl.Segments {
		if seg == nil {
			continue
		}
		if seg.Map != nil {
			return false
		}
		if strings.HasSuffix(seg.URI, ".m4s") || strings.HasSuffix(seg.URI, ".mp4") {
			return false
		}
		if strings.HasSuffix(seg.URI, ".ts") {
			return true
		}
	}
	return false
}
