// Package drmlocal implements a local Widevine CDM adapter (PLAN.md §6.6).
// It uses the gowidevine Go library to perform license acquisition and
// decryption in-process. The adapter loads a .wvd device file and satisfies
// the drm.Client interface.
//
// The .wvd file contains the device's RSA private key, client identification,
// and other provisioning data. It is a sensitive credential and must be
// stored outside the repository (PLAN.md §2.4).
package drmlocal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	widevine "github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"
	"github.com/nem-git/abcmovies/core/drm"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Adapter is the local Widevine CDM adapter. It loads a .wvd device file
// and performs license acquisition and decryption in-process using the
// gowidevine library.
type Adapter struct {
	logger *slog.Logger
	device *widevine.Device
	cdm    *widevine.CDM
	http   *http.Client
}

// NewAdapter loads a .wvd device file and creates a local Widevine CDM
// adapter. The wvdPath must point to a valid .wvd file outside the repo.
func NewAdapter(wvdPath string, logger *slog.Logger) (*Adapter, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if wvdPath == "" {
		return nil, fmt.Errorf("drmlocal: wvd_path is required")
	}

	f, err := os.Open(wvdPath)
	if err != nil {
		return nil, fmt.Errorf("drmlocal: open wvd: %w", err)
	}
	defer func() { _ = f.Close() }()

	device, err := widevine.NewDevice(widevine.FromWVD(f))
	if err != nil {
		return nil, fmt.Errorf("drmlocal: load wvd: %w", err)
	}

	return &Adapter{
		logger: logger,
		device: device,
		cdm:    widevine.NewCDM(device),
		http:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Supports reports whether this adapter can handle the given DRM system.
// The local CDM adapter supports Widevine only.
func (a *Adapter) Supports(system slotsv1.DrmSystem) bool {
	return system == slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE
}

// AcquireKeys acquires content keys for an encrypted track using the local
// CDM. It parses the PSSH, generates a license challenge, sends it to the
// license server, and extracts the content keys from the license response.
func (a *Adapter) AcquireKeys(ctx context.Context, providerID, contentID string, system slotsv1.DrmSystem, pssh []byte, keyIDs []string, licenseURL, authContext string) ([]*slotsv1.ContentKey, error) {
	if len(pssh) == 0 {
		return nil, fmt.Errorf("drmlocal: pssh is required")
	}
	if licenseURL == "" {
		return nil, fmt.Errorf("drmlocal: license_url is required")
	}

	// Parse PSSH
	parsedPSSH, err := widevine.NewPSSH(pssh)
	if err != nil {
		return nil, fmt.Errorf("drmlocal: parse pssh: %w", err)
	}

	// Get license challenge
	challenge, parseLicense, err := a.cdm.GetLicenseChallenge(
		parsedPSSH,
		widevinepb.LicenseType_STREAMING,
		false, // privacy mode — not needed for public test servers
	)
	if err != nil {
		return nil, fmt.Errorf("drmlocal: get license challenge: %w", err)
	}

	// Send challenge to license server
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, licenseURL, bytes.NewReader(challenge))
	if err != nil {
		return nil, fmt.Errorf("drmlocal: create license request: %w", err)
	}
	if authContext != "" {
		req.Header.Set("Authorization", authContext)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("drmlocal: license request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("drmlocal: license server returned %s", resp.Status)
	}

	licenseResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("drmlocal: read license response: %w", err)
	}

	// Parse license response
	keys, err := parseLicense(licenseResp)
	if err != nil {
		return nil, fmt.Errorf("drmlocal: parse license: %w", err)
	}

	// Convert to ContentKey
	out := make([]*slotsv1.ContentKey, 0, len(keys))
	for _, k := range keys {
		if k.Type != widevinepb.License_KeyContainer_CONTENT {
			continue
		}
		out = append(out, &slotsv1.ContentKey{
			KeyId:       fmt.Sprintf("%x", k.ID),
			KeyMaterial: k.Key,
			Validity:    nil, // validity comes from the license, not the key
		})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("drmlocal: no content keys in license response")
	}

	a.logger.Info("drmlocal: acquired keys",
		"provider", providerID,
		"content", contentID,
		"system", system,
		"keys", len(out),
	)
	return out, nil
}

// Decrypt decrypts an encrypted track source into clear bytes using the
// content keys. It uses gowidevine's DecryptMP4Auto for in-process MP4
// decryption.
func (a *Adapter) Decrypt(ctx context.Context, source drm.Source, keys []*slotsv1.ContentKey) ([]byte, error) {
	if len(source.InlineBytes) == 0 {
		return nil, fmt.Errorf("drmlocal: inline_bytes is required")
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("drmlocal: at least one key is required")
	}

	// Convert ContentKey to gowidevine Key
	wvKeys := make([]*widevine.Key, 0, len(keys))
	for _, k := range keys {
		wvKeys = append(wvKeys, &widevine.Key{
			ID:  []byte(k.GetKeyId()),
			Key: k.GetKeyMaterial(),
		})
	}

	// Decrypt MP4
	var buf bytes.Buffer
	if err := widevine.DecryptMP4Auto(bytes.NewReader(source.InlineBytes), wvKeys, &buf); err != nil {
		return nil, fmt.Errorf("drmlocal: decrypt: %w", err)
	}

	a.logger.Debug("drmlocal: decrypted", "bytes", buf.Len())
	return buf.Bytes(), nil
}
