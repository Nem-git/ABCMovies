// Package convert provides on-the-fly stream format conversion.
// Converters transmux one streaming format into another (e.g. DASH -> MP4)
// using a shared Fetcher to pull upstream manifests and segments.
package convert

import (
	"context"
	"io"

	"github.com/nem-git/abcmovies/internal/stream"
)

// Converter transmuxes a source stream into a target format.
// Implementations are expected to be pure-Go remuxers; the interface is kept
// minimal so an FFmpeg-based converter can be dropped in later.
type Converter interface {
	// Supports reports whether the converter can convert between the given
	// source and target stream formats.
	Supports(sourceFormat, targetFormat string) bool

	// Convert transmuxes the stream described by src into w.
	// src must point at the stream manifest (DASH MPD or HLS playlist).
	Convert(ctx context.Context, src *stream.Locator, w io.Writer) error
}

// Key builds the registry key for a source -> target conversion.
func Key(sourceFormat, targetFormat string) string {
	return sourceFormat + "->" + targetFormat
}

// Registry maps "source->target" conversion keys to converters.
type Registry struct {
	converters map[string]Converter
}

// newRegistry returns an empty registry.
func newRegistry() *Registry {
	return &Registry{converters: make(map[string]Converter)}
}

// Register associates a converter with a source -> target conversion.
func (r *Registry) Register(sourceFormat, targetFormat string, c Converter) {
	r.converters[Key(sourceFormat, targetFormat)] = c
}

// Get returns the converter registered for sourceFormat -> targetFormat.
func (r *Registry) Get(sourceFormat, targetFormat string) (Converter, bool) {
	if r == nil {
		return nil, false
	}
	c, ok := r.converters[Key(sourceFormat, targetFormat)]
	return c, ok
}
