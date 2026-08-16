package drm

import (
	"context"
)

// AES128Config configures the HLS AES-128 key provider.
type AES128Config struct {
	// Enabled turns on AES-128 key retrieval.
	Enabled bool `yaml:"enabled"`
	// Headers are extra headers for fetching the key.
	Headers map[string]string
}

// AES128Provider serves HLS AES-128 segment keys. Keys come from EXT-X-KEY
// URIs (already proxied by the HLS strategy); the request's KIDs carry the key
// URI resolved from the playlist.
type AES128Provider struct {
	cfg    AES128Config
	client *licenseClient
}

// NewAES128 returns an AES-128 key provider.
func NewAES128(cfg AES128Config) (*AES128Provider, error) {
	return &AES128Provider{cfg: cfg, client: newLicenseClient()}, nil
}

// Scheme implements KeyProvider.
func (p *AES128Provider) Scheme() Scheme {
	return SchemeAES128
}

// GetKeys fetches the key at the requested URI. For AES-128 the license URL is
// the key URI itself and there is one key per stream, so we return it for the
// first requested KID.
func (p *AES128Provider) GetKeys(ctx context.Context, req KeyRequest) (map[KID]CEK, error) {
	if req.LicenseURL == "" {
		return nil, ErrNotConfigured
	}
	headers := headersFromMap(req.Headers, p.cfg.Headers)
	key, err := p.client.post(ctx, req.LicenseURL, httpHeaders(headers), nil)
	if err != nil {
		return nil, err
	}
	out := make(map[KID]CEK)
	var assigned KID
	for _, kid := range req.KIDs {
		if assigned == kid && len(out) > 0 {
			continue
		}
		out[kid] = append(CEK(nil), key...)
		assigned = kid
		break
	}
	if len(out) == 0 {
		return nil, ErrKIDNotFound
	}
	return out, nil
}
