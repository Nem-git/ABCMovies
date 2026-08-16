package drm

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// Engine decrypts fMP4 streams protected with CENC/CBCS using the vendored
// mp4ff. It resolves keys through the KeyProvider registered for the scheme
// detected in the init segment (typically a vault-wrapped provider).
type Engine struct {
	providers map[Scheme]KeyProvider
}

// NewEngine returns an Engine that resolves keys via provider for the
// provider's scheme.
func NewEngine(provider KeyProvider) *Engine {
	e := &Engine{providers: make(map[Scheme]KeyProvider)}
	if provider != nil {
		e.providers[provider.Scheme()] = provider
	}
	return e
}

// NewEngineSet returns an Engine that resolves keys via the provider registered
// for each scheme.
func NewEngineSet(providers map[Scheme]KeyProvider) *Engine {
	e := &Engine{providers: make(map[Scheme]KeyProvider, len(providers))}
	for s, p := range providers {
		if p != nil {
			e.providers[s] = p
		}
	}
	return e
}

// Stream is a prepared stream: the cleaned init segment plus per-track decrypt
// info and the resolved keys for all encrypted tracks.
type Stream struct {
	// CleanInit is the init segment with PSSH/sinf/tenc stripped (per DRM.md).
	CleanInit *mp4.InitSegment
	di        mp4.DecryptInfo
	keysByKID map[string][]byte
}

// PrepareInit cleans an init segment and resolves keys for its encrypted
// tracks. The resulting Stream decrypts media segments for this init.
func (e *Engine) PrepareInit(ctx context.Context, init *mp4.InitSegment, req KeyRequest) (*Stream, error) {
	if init == nil || init.Moov == nil {
		return nil, errors.New("drm: nil init segment")
	}

	// Identify encrypted tracks and collect their KIDs.
	kids := kidsFromInit(init)
	if len(kids) == 0 {
		// No encrypted tracks: pass through untouched.
		return &Stream{CleanInit: init}, nil
	}
	if len(e.providers) == 0 {
		return nil, ErrNotConfigured
	}

	// Detect the scheme from the init's PSSH boxes (falling back to the
	// caller-provided scheme so ClearKey/AES-128 can be requested explicitly).
	scheme := req.Scheme
	if scheme == "" {
		scheme = schemeFromInit(init)
	}
	if scheme == "" {
		return nil, fmt.Errorf("drm: %w: no PSSH in init segment", ErrUnsupported)
	}
	provider, ok := e.providers[scheme]
	if !ok {
		return nil, fmt.Errorf("drm: %w: %s", ErrNotConfigured, scheme)
	}

	// Fill in anything the request leaves blank: PSSH payload, KIDs, content ID.
	req.Scheme = scheme
	req.KIDs = kids
	if len(req.PSSH) == 0 {
		req.PSSH = psshPayloadForScheme(init, scheme)
	}
	if len(req.ContentID) == 0 {
		if info, err := extractInitProtectInfo(init); err == nil {
			req.ContentID = info.ContentID
		}
	}

	// DecryptInit strips pssh/sinf/tenc from the init and returns track info.
	di, err := mp4.DecryptInit(init)
	if err != nil {
		return nil, fmt.Errorf("drm: decrypt init: %w", err)
	}

	// Resolve keys.
	keys, err := provider.GetKeys(ctx, req)
	if err != nil {
		return nil, err
	}
	keysByKID := make(map[string][]byte, len(keys))
	for kid, cek := range keys {
		keysByKID[kid.String()] = []byte(cek)
	}

	return &Stream{CleanInit: init, di: di, keysByKID: keysByKID}, nil
}

// DecryptSegment decrypts a media segment in place and returns it re-encoded.
func (s *Stream) DecryptSegment(data []byte) ([]byte, error) {
	if len(s.keysByKID) == 0 {
		return data, nil
	}
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	if err != nil {
		return nil, fmt.Errorf("drm: decode media segment: %w", err)
	}
	if len(f.Segments) != 1 {
		return nil, fmt.Errorf("drm: media segment contains %d segments, expected 1", len(f.Segments))
	}
	seg := f.Segments[0]
	if err := mp4.DecryptSegmentWithKeys(seg, s.di, nil, s.keysByKID, false); err != nil {
		return nil, fmt.Errorf("drm: decrypt segment: %w", err)
	}
	var buf bytes.Buffer
	if err := seg.Encode(&buf); err != nil {
		return nil, fmt.Errorf("drm: encode decrypted segment: %w", err)
	}
	return buf.Bytes(), nil
}
