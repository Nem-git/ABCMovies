// Package drm provides content key acquisition and stream decryption for
// Widevine, PlayReady, ClearKey and AES-128 protected content.
//
// The package is split in two layers:
//
//   - Key providers (KeyProvider) turn a KeyRequest (PSSH + KIDs) into
//     content keys (KID -> CEK). Implementations exist for Widevine (via
//     diana or gowidevine), PlayReady (via diana), ClearKey and AES-128.
//   - A Vault wraps a KeyProvider with an in-memory cache (TTL + singleflight)
//     so repeat key requests never hit the license server.
//   - The Engine uses the vendored mp4ff to decrypt CENC/CBCS fMP4 init and
//     media segments given the resolved keys.
package drm

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

// Scheme identifies a DRM system.
type Scheme string

const (
	SchemeWidevine  Scheme = "widevine"
	SchemePlayReady Scheme = "playready"
	SchemeClearKey  Scheme = "clearkey"
	SchemeAES128    Scheme = "aes128"
)

// KID is a 16-byte content key ID (the UUID carried in a tenc box or PSSH).
type KID [16]byte

// String returns the 32-char lowercase hex form used by mp4ff and license maps.
func (k KID) String() string {
	return hex.EncodeToString(k[:])
}

// CEK is a content encryption key (16 bytes for AES-128).
type CEK []byte

// KeyRequest describes the keys needed for a protected stream.
type KeyRequest struct {
	// ProviderTag identifies the upstream provider, used for vault scoping.
	ProviderTag string
	// ContentKey identifies the content (e.g. "movie:123"), used for vault scoping.
	ContentKey string
	// Scheme is the DRM system to license from.
	Scheme Scheme
	// PSSH is the raw protection-system-specific header for the stream.
	// For Widevine this is the pssh payload (without the pssh box wrapper);
	// for PlayReady it is the PRO XML document.
	PSSH []byte
	// KIDs are the key IDs requested. When empty, providers derive them from PSSH.
	KIDs []KID
	// ContentID is an optional content identifier used by some license servers.
	ContentID []byte
	// LicenseURL is the license server endpoint. Empty when keys are static.
	LicenseURL string
	// Headers are extra headers to send with the license request (auth tokens etc).
	Headers http.Header
}

// Keys reports whether the request names at least one KID or carries a PSSH.
func (r KeyRequest) Keys() bool {
	return len(r.KIDs) > 0 || len(r.PSSH) > 0
}

// KeyProvider acquires content keys for a DRM scheme.
type KeyProvider interface {
	// Scheme returns the DRM scheme this provider licenses.
	Scheme() Scheme
	// GetKeys returns a KID -> CEK map for the request.
	GetKeys(ctx context.Context, req KeyRequest) (map[KID]CEK, error)
}

// Errors surfaced by providers and the engine.
var (
	ErrNotConfigured = errors.New("drm: provider not configured")
	ErrDeviceMissing = errors.New("drm: device files missing")
	ErrPSSHNotFound  = errors.New("drm: pssh not found")
	ErrKIDNotFound   = errors.New("drm: key id not found")
	ErrCEKNotFound   = errors.New("drm: content key not found")
	ErrEmptyLicense  = errors.New("drm: license contained no keys")
	ErrUnsupported   = errors.New("drm: scheme not supported")
)

// licenseTTL is the default time keys stay valid in the vault.
const licenseTTL = 12 * time.Hour

// licenseTimeout bounds a single license request.
const licenseTimeout = 30 * time.Second
