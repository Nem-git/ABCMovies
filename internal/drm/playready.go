package drm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"41.neocities.org/diana/playReady"
)

// PlayReadyConfig configures the PlayReady key provider.
type PlayReadyConfig struct {
	// DeviceDir is the directory containing the PlayReady device files
	// (bdevcert.dat, zprivsig.dat, zprivencr.dat).
	DeviceDir string
	// Headers are extra headers for all license requests.
	Headers map[string]string
}

// PlayReadyProvider acquires PlayReady keys via the diana pure-Go CDM.
// It implements KeyProvider.
type PlayReadyProvider struct {
	cfg    PlayReadyConfig
	client *licenseClient
}

// NewPlayReady returns a PlayReady key provider from cfg.
func NewPlayReady(cfg PlayReadyConfig) (*PlayReadyProvider, error) {
	if cfg.DeviceDir == "" {
		return nil, fmt.Errorf("%w: playready device dir", ErrNotConfigured)
	}
	return &PlayReadyProvider{cfg: cfg, client: newLicenseClient()}, nil
}

// Scheme implements KeyProvider.
func (p *PlayReadyProvider) Scheme() Scheme {
	return SchemePlayReady
}

// GetKeys licenses the requested KIDs and returns KID -> CEK.
func (p *PlayReadyProvider) GetKeys(ctx context.Context, req KeyRequest) (map[KID]CEK, error) {
	headers := headersFromMap(req.Headers, p.cfg.Headers)
	out := make(map[KID]CEK)

	for _, kid := range req.KIDs {
		key, err := p.keyForKID(ctx, kid, req, headers)
		if err != nil {
			return nil, err
		}
		out[kid] = key
	}
	if len(out) == 0 {
		return nil, ErrKIDNotFound
	}
	return out, nil
}

// keyForKID performs one PlayReady license acquisition for a single key ID.
func (p *PlayReadyProvider) keyForKID(ctx context.Context, kid KID, req KeyRequest, headers http.Header) (CEK, error) {
	// Load device chain and keys (maya's layout).
	chainData, err := os.ReadFile(filepath.Join(p.cfg.DeviceDir, "bdevcert.dat"))
	if err != nil {
		return nil, fmt.Errorf("%w: reading bdevcert.dat: %v", ErrDeviceMissing, err)
	}
	chain, err := playReady.ParseChain(chainData)
	if err != nil {
		return nil, fmt.Errorf("drm: parse playready chain: %w", err)
	}

	signData, err := os.ReadFile(filepath.Join(p.cfg.DeviceDir, "zprivsig.dat"))
	if err != nil {
		return nil, fmt.Errorf("%w: reading zprivsig.dat: %v", ErrDeviceMissing, err)
	}
	signingKey, err := playReady.ParseRawPrivateKey(signData)
	if err != nil {
		return nil, fmt.Errorf("drm: parse playready signing key: %w", err)
	}

	encData, err := os.ReadFile(filepath.Join(p.cfg.DeviceDir, "zprivencr.dat"))
	if err != nil {
		return nil, fmt.Errorf("%w: reading zprivencr.dat: %v", ErrDeviceMissing, err)
	}
	encryptKey, err := playReady.ParseRawPrivateKey(encData)
	if err != nil {
		return nil, fmt.Errorf("drm: parse playready encrypt key: %w", err)
	}

	contentID := string(req.ContentID)
	if contentID == "" && len(req.PSSH) > 0 {
		if id, err := playReadyContentID(req.PSSH); err == nil && len(id) > 0 {
			contentID = string(id)
		}
	}

	playReady.UuidOrGuid(kid[:])
	reqData, err := chain.LicenseRequestBytes(signingKey, kid[:], contentID)
	if err != nil {
		return nil, fmt.Errorf("drm: build playready request: %w", err)
	}

	resp, err := p.client.post(ctx, req.LicenseURL, httpHeaders(headers), reqData)
	if err != nil {
		return nil, err
	}
	license, err := playReady.ParseLicense(resp)
	if err != nil {
		return nil, fmt.Errorf("drm: parse playready license: %w", err)
	}

	if license.ContainerOuter.ContainerKeys.ContentKey.GuidKeyID != nil {
		if !equalBytes(license.ContainerOuter.ContainerKeys.ContentKey.GuidKeyID, kid[:]) {
			return nil, errors.New("drm: playready key id mismatch")
		}
	}

	key, err := license.Decrypt(encryptKey)
	if err != nil {
		return nil, fmt.Errorf("drm: decrypt playready license: %w", err)
	}
	if len(key) == 16 && equalBytes(key, make([]byte, 16)) {
		return nil, errors.New("drm: zero key received")
	}
	return append(CEK(nil), key...), nil
}
