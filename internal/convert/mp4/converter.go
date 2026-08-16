// Package mp4 converts DASH and HLS streams to a single fragmented MP4 file.
//
// Supported inputs:
//
//   - DASH (application/dash+xml): picks the highest-bandwidth representation
//     per adaptation set and either byte-concatenates the init + media segments
//     (single track) or merges tracks with mp4ff (separate video/audio sets).
//   - HLS (application/vnd.apple.mpegurl): picks the highest-bandwidth variant
//     (plus its default audio rendition) and handles both fMP4 and MPEG-TS
//     segment formats.
package mp4

import (
	"context"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"

	"github.com/nem-git/abcmovies/internal/drm"
	"github.com/nem-git/abcmovies/internal/proxy"
	"github.com/nem-git/abcmovies/internal/stream"
)

// Format constants used for converter dispatch.
const (
	FormatDASH = "dash"
	FormatHLS  = "hls"
	FormatMP4  = "mp4"

	mimeDASH = "application/dash+xml"
	mimeHLS  = "application/vnd.apple.mpegurl"
	mimeMP4  = "video/mp4"
)

// Converter converts DASH and HLS streams to MP4.
type Converter struct {
	fetcher proxy.Fetcher
	drm     *drm.Engine
}

// NewConverter returns a Converter that fetches upstream content via fetcher.
func NewConverter(fetcher proxy.Fetcher) *Converter {
	return &Converter{fetcher: fetcher}
}

// NewConverterWithDRM returns a Converter that decrypts DRM-protected init and
// media segments with engine before transmuxing.
func NewConverterWithDRM(fetcher proxy.Fetcher, engine *drm.Engine) *Converter {
	return &Converter{fetcher: fetcher, drm: engine}
}

// Supports reports whether the converter can convert sourceFormat -> targetFormat.
func (c *Converter) Supports(sourceFormat, targetFormat string) bool {
	return targetFormat == FormatMP4 &&
		(sourceFormat == FormatDASH || sourceFormat == FormatHLS)
}

// Convert transmuxes the stream described by src into w.
func (c *Converter) Convert(ctx context.Context, src *stream.Locator, w io.Writer) error {
	if src == nil {
		return fmt.Errorf("convert: nil source locator")
	}
	switch src.EncodingFormat {
	case mimeDASH:
		return c.convertDASH(ctx, src, w)
	case mimeHLS:
		return c.convertHLS(ctx, src, w)
	case mimeMP4:
		// Already MP4: stream it through unchanged.
		body, _, err := c.fetcher.Fetch(ctx, src.URL, src.Headers, src.Query)
		if err != nil {
			return err
		}
		defer body.Close()
		_, err = io.Copy(w, body)
		return err
	default:
		return fmt.Errorf("convert: unsupported source encoding format %q", src.EncodingFormat)
	}
}

// fmp4Track is a single input stream of fMP4 init + media segments.
type fmp4Track struct {
	label       string
	initURL     string
	segmentURLs []string
}

// writeFMP4 writes a combined fragmented MP4 from one or more input tracks.
func (c *Converter) writeFMP4(ctx context.Context, src *stream.Locator, tracks []fmp4Track, w io.Writer) error {
	switch {
	case len(tracks) == 0:
		return fmt.Errorf("convert: no tracks found in source")
	case len(tracks) == 1:
		return c.writeFMP4Concat(ctx, src, tracks[0], w)
	default:
		return c.writeFMP4Merge(ctx, src, tracks, w)
	}
}

// writeFMP4Concat byte-concatenates init + media segments. Valid when a single
// track carries all media (video only, or audio+video multiplexed).
func (c *Converter) writeFMP4Concat(ctx context.Context, src *stream.Locator, t fmp4Track, w io.Writer) error {
	if t.initURL == "" {
		return fmt.Errorf("convert: no init segment for %s", t.label)
	}
	initData, err := c.fetch(ctx, src, t.initURL)
	if err != nil {
		return fmt.Errorf("convert: fetching init segment %s: %w", t.initURL, err)
	}
	if c.drm != nil {
		init, err := decodeInitSegment(initData)
		if err != nil {
			return fmt.Errorf("convert: decoding init segment %s: %w", t.initURL, err)
		}
		stream, err := c.drm.PrepareInit(ctx, init, c.keyRequest(src))
		if err != nil {
			return fmt.Errorf("convert: preparing init segment %s: %w", t.initURL, err)
		}
		if err := stream.CleanInit.Encode(w); err != nil {
			return fmt.Errorf("convert: encoding init segment %s: %w", t.initURL, err)
		}
		for _, u := range t.segmentURLs {
			data, err := c.fetch(ctx, src, u)
			if err != nil {
				return fmt.Errorf("convert: fetching media segment %s: %w", u, err)
			}
			dec, err := stream.DecryptSegment(data)
			if err != nil {
				return fmt.Errorf("convert: decrypting media segment %s: %w", u, err)
			}
			if _, err := w.Write(dec); err != nil {
				return fmt.Errorf("convert: writing media segment %s: %w", u, err)
			}
		}
		return nil
	}
	if err := c.fetchTo(ctx, src, t.initURL, w); err != nil {
		return fmt.Errorf("convert: fetching init segment %s: %w", t.initURL, err)
	}
	for _, u := range t.segmentURLs {
		if err := c.fetchTo(ctx, src, u, w); err != nil {
			return fmt.Errorf("convert: fetching media segment %s: %w", u, err)
		}
	}
	return nil
}

// writeFMP4Merge merges multiple single-track inputs into one fragmented MP4,
// renumbering track IDs and interleaving one multi-track fragment per segment.
func (c *Converter) writeFMP4Merge(ctx context.Context, src *stream.Locator, tracks []fmp4Track, w io.Writer) error {
	numSegments := len(tracks[0].segmentURLs)
	newTrackIDs := make([]uint32, len(tracks))
	trexes := make([]*mp4.TrexBox, len(tracks))
	drms := make([]*drm.Stream, len(tracks))

	var combinedInit *mp4.InitSegment
	for i, t := range tracks {
		if len(t.segmentURLs) != numSegments {
			return fmt.Errorf("convert: %s has %d segments, expected %d (tracks must be segment-aligned)", t.label, len(t.segmentURLs), numSegments)
		}
		data, err := c.fetch(ctx, src, t.initURL)
		if err != nil {
			return fmt.Errorf("convert: fetching init segment %s: %w", t.initURL, err)
		}
		init, err := decodeInitSegment(data)
		if err != nil {
			return fmt.Errorf("convert: decoding init segment %s: %w", t.initURL, err)
		}
		if c.drm != nil {
			stream, err := c.drm.PrepareInit(ctx, init, c.keyRequest(src))
			if err != nil {
				return fmt.Errorf("convert: preparing init segment %s: %w", t.initURL, err)
			}
			drms[i] = stream
			init = stream.CleanInit
		}
		id := uint32(i + 1)
		init.Moov.Trak.Tkhd.TrackID = id
		if init.Moov.Mvex != nil && init.Moov.Mvex.Trex != nil {
			init.Moov.Mvex.Trex.TrackID = id
			trexes[i] = init.Moov.Mvex.Trex
		}
		if combinedInit == nil {
			combinedInit = init
		} else {
			combinedInit.Moov.AddChild(init.Moov.Trak)
			if init.Moov.Mvex != nil {
				if init.Moov.Mvex.Trex != nil {
					combinedInit.Moov.Mvex.AddChild(init.Moov.Mvex.Trex)
				}
				if init.Moov.Mvex.Mehd != nil {
					combinedInit.Moov.Mvex.AddChild(init.Moov.Mvex.Mehd)
				}
			}
		}
		newTrackIDs[i] = id
	}
	combinedInit.Moov.Mvhd.NextTrackID = uint32(len(tracks) + 1)
	if err := combinedInit.Encode(w); err != nil {
		return fmt.Errorf("convert: encoding combined init segment: %w", err)
	}

	for idx := 0; idx < numSegments; idx++ {
		var combinedSeg *mp4.MediaSegment
		var outFrag *mp4.Fragment
		for i, t := range tracks {
			data, err := c.fetch(ctx, src, t.segmentURLs[idx])
			if err != nil {
				return fmt.Errorf("convert: fetching media segment %s: %w", t.segmentURLs[idx], err)
			}
			if drms[i] != nil {
				data, err = drms[i].DecryptSegment(data)
				if err != nil {
					return fmt.Errorf("convert: decrypting media segment %s: %w", t.segmentURLs[idx], err)
				}
			}
			f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
			if err != nil {
				return fmt.Errorf("convert: decoding media segment %s: %w", t.segmentURLs[idx], err)
			}
			if len(f.Segments) != 1 {
				return fmt.Errorf("convert: media segment %s contains %d segments, expected 1", t.segmentURLs[idx], len(f.Segments))
			}
			seg := f.Segments[0]
			if len(seg.Fragments) != 1 {
				return fmt.Errorf("convert: media segment %s contains %d fragments, expected 1", t.segmentURLs[idx], len(seg.Fragments))
			}
			frag := seg.Fragments[0]
			if i == 0 {
				outFrag, err = mp4.CreateMultiTrackFragment(frag.Moof.Mfhd.SequenceNumber, newTrackIDs)
				if err != nil {
					return fmt.Errorf("convert: creating merged fragment: %w", err)
				}
				if seg.Styp != nil {
					combinedSeg = mp4.NewMediaSegmentWithStyp(seg.Styp)
				} else {
					combinedSeg = mp4.NewMediaSegmentWithoutStyp()
				}
				combinedSeg.AddFragment(outFrag)
			}
			fss, err := frag.GetFullSamples(trexes[i])
			if err != nil {
				return fmt.Errorf("convert: extracting samples from %s: %w", t.segmentURLs[idx], err)
			}
			for _, fs := range fss {
				if err := outFrag.AddFullSampleToTrack(fs, newTrackIDs[i]); err != nil {
					return fmt.Errorf("convert: merging samples from %s: %w", t.segmentURLs[idx], err)
				}
			}
		}
		if err := combinedSeg.Encode(w); err != nil {
			return fmt.Errorf("convert: encoding merged media segment %d: %w", idx, err)
		}
	}
	return nil
}

// keyRequest builds the DRM key request scope for a conversion. The engine
// fills in the scheme, PSSH, KIDs and content ID from the init segment.
func (c *Converter) keyRequest(src *stream.Locator) drm.KeyRequest {
	return drm.KeyRequest{
		ProviderTag: src.ProviderTag,
		ContentKey:  src.ContentKey,
		Headers:     src.Headers,
	}
}

// decodeInitSegment decodes a standalone fMP4 init segment (ftyp+moov).
func decodeInitSegment(data []byte) (*mp4.InitSegment, error) {
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	if err != nil {
		return nil, err
	}
	init := f.Init
	if init == nil || init.Moov == nil {
		return nil, fmt.Errorf("no init segment (moov) found")
	}
	if len(init.Moov.Traks) != 1 {
		return nil, fmt.Errorf("init segment has %d tracks, expected 1", len(init.Moov.Traks))
	}
	return init, nil
}

// fetch retrieves an upstream resource into memory.
func (c *Converter) fetch(ctx context.Context, src *stream.Locator, rawURL string) ([]byte, error) {
	body, _, err := c.fetcher.Fetch(ctx, rawURL, src.Headers, src.Query)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

// fetchTo copies an upstream resource into w.
func (c *Converter) fetchTo(ctx context.Context, src *stream.Locator, rawURL string, w io.Writer) error {
	body, _, err := c.fetcher.Fetch(ctx, rawURL, src.Headers, src.Query)
	if err != nil {
		return err
	}
	defer body.Close()
	_, err = io.Copy(w, body)
	return err
}
