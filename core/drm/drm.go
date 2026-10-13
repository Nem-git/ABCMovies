// Package drm defines the DRM slot's contract surface that the delivery
// engine consumes (PLAN.md §6.6). It is a narrow, importable package so
// both the engine (core/internal/delivery) and external adapters can share
// the same types without an import cycle.
//
// The Client interface is satisfied by an admitted DRM slot through a
// narrow adapter. The Source struct carries the encrypted source for a
// Decrypt call.
package drm

import (
	"context"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Client is the DRM slot's contract surface the delivery engine consumes
// (PLAN.md §6.6). It is satisfied by an admitted DRM slot through a narrow
// adapter. The engine uses it to acquire keys and decrypt tracks before
// delivery (§6.2: "DRM before anything").
type Client interface {
	// Supports reports whether this DRM client can handle the given DRM
	// system (PLAN.md §6.6). The engine calls this at resolve time to
	// decide whether the pipeline includes a decrypt step.
	Supports(system slotsv1.DrmSystem) bool
	// AcquireKeys asks the DRM slot to acquire content keys for an encrypted
	// track (PLAN.md §6.6). licenseURL is the endpoint to send the license
	// challenge to; authContext carries the credentials for that call.
	AcquireKeys(ctx context.Context, providerID, contentID string, system slotsv1.DrmSystem, pssh []byte, keyIDs []string, licenseURL, authContext string) ([]*slotsv1.ContentKey, error)
	// Decrypt asks the DRM slot to decrypt an encrypted track source into
	// clear bytes (PLAN.md §6.6).
	Decrypt(ctx context.Context, source Source, keys []*slotsv1.ContentKey) ([]byte, error)
}

// Source carries the encrypted source for a Decrypt call (PLAN.md §6.6).
// InlineBytes is what M8 uses — the engine fetches the encrypted source
// and passes it directly. SourceRef is reserved so the relay-URL variant
// (streaming decrypt) can be added later without changing this interface.
type Source struct {
	// InlineBytes is the encrypted source, already fetched by the engine.
	InlineBytes []byte
	// SourceRef is a reference to the encrypted source (e.g. a relay URL)
	// that the slot fetches and decrypts. Reserved for a later milestone.
	SourceRef string
}
