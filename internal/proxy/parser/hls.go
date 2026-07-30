package parser

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/Eyevinn/hls-m3u8/m3u8"
)

type (
	MasterPlaylist  = m3u8.MasterPlaylist
	MediaPlaylist   = m3u8.MediaPlaylist
	Variant         = m3u8.Variant
	VariantParams   = m3u8.VariantParams
	Alternative     = m3u8.Alternative
	MediaSegment    = m3u8.MediaSegment
	Map             = m3u8.Map
	Key             = m3u8.Key
	PartialSegment  = m3u8.PartialSegment
	PreloadHint     = m3u8.PreloadHint
	SessionData     = m3u8.SessionData
	ContentSteering = m3u8.ContentSteering
)

// HLSParser handles HLS playlist parsing and encoding.
type HLSParser struct{}

// Decode auto-detects master vs media playlist and returns both (one will be nil).
func (p *HLSParser) Decode(data []byte) (*MasterPlaylist, *MediaPlaylist, error) {
	playlist, listType, err := m3u8.Decode(*bytes.NewBuffer(data), false)
	if err != nil {
		return nil, nil, err
	}
	switch listType {
	case m3u8.MASTER:
		return playlist.(*MasterPlaylist), nil, nil
	case m3u8.MEDIA:
		return nil, playlist.(*MediaPlaylist), nil
	default:
		return nil, nil, nil
	}
}

// EncodeMaster encodes a master playlist to bytes.
func (p *HLSParser) EncodeMaster(pl *MasterPlaylist) ([]byte, error) {
	buf := pl.Encode()
	return buf.Bytes(), nil
}

// EncodeMedia encodes a media playlist to bytes.
func (p *HLSParser) EncodeMedia(pl *MediaPlaylist) ([]byte, error) {
	buf := pl.Encode()
	return buf.Bytes(), nil
}

// SyntheticMaster creates a master playlist with a single variant pointing to the given URI.
func SyntheticMaster(variantURI string) *MasterPlaylist {
	pl := m3u8.NewMasterPlaylist()
	pl.Variants = append(pl.Variants, &Variant{
		URI: variantURI,
		VariantParams: VariantParams{
			Bandwidth: 1,
		},
	})
	return pl
}

// ResolveURL resolves a potentially relative URL against a base URL.
func ResolveURL(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return baseURL.ResolveReference(refURL).String()
}
