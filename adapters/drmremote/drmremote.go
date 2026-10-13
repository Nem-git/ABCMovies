// Package drmremote implements a remote key provider adapter (PLAN.md §6.6).
// It calls a pywidevine-compatible HTTP service for key acquisition, then
// decrypts locally using the gowidevine library.
//
// The pywidevine service runs as a separate Python process. It exposes a
// REST API for CDM operations (open session, get license challenge, parse
// license, get keys). The adapter is a thin HTTP client that orchestrates
// these calls.
//
// Decryption happens locally (not in the remote service) because:
// 1. pywidevine's decrypt is file-based (shells out to shaka-packager)
// 2. Our Opener works with bytes, not file paths
// 3. Transferring large encrypted files to/from the remote service is wasteful
package drmremote

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	widevine "github.com/iyear/gowidevine"
	"github.com/nem-git/abcmovies/core/drm"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// Adapter is the remote key provider adapter. It calls a pywidevine-
// compatible HTTP service for key acquisition, then decrypts locally
// using gowidevine.
type Adapter struct {
	serviceURL string
	secretKey  string
	deviceName string
	logger     *slog.Logger
	http       *http.Client
}

// NewAdapter creates a remote key provider adapter. The serviceURL is the
// pywidevine service's base URL; secretKey is the X-Secret-Key header value
// for authentication; deviceName is the device identifier in the service.
func NewAdapter(serviceURL, secretKey, deviceName string, logger *slog.Logger) (*Adapter, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if serviceURL == "" {
		return nil, fmt.Errorf("drmremote: service_url is required")
	}
	if deviceName == "" {
		return nil, fmt.Errorf("drmremote: device_name is required")
	}
	return &Adapter{
		serviceURL: serviceURL,
		secretKey:  secretKey,
		deviceName: deviceName,
		logger:     logger,
		http:       &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Supports reports whether this adapter can handle the given DRM system.
// The remote adapter supports Widevine only.
func (a *Adapter) Supports(system slotsv1.DrmSystem) bool {
	return system == slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE
}

// apiResponse is the standard JSON response envelope from the pywidevine
// service: {status, message, data}.
type apiResponse struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// AcquireKeys acquires content keys for an encrypted track by calling the
// pywidevine service. It opens a CDM session, gets a license challenge,
// sends it to the license server, parses the response, and returns the
// content keys.
func (a *Adapter) AcquireKeys(ctx context.Context, providerID, contentID string, system slotsv1.DrmSystem, pssh []byte, keyIDs []string, licenseURL, authContext string) ([]*slotsv1.ContentKey, error) {
	if len(pssh) == 0 {
		return nil, fmt.Errorf("drmremote: pssh is required")
	}
	if licenseURL == "" {
		return nil, fmt.Errorf("drmremote: license_url is required")
	}

	// Open a CDM session
	sessionID, err := a.openSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("drmremote: open session: %w", err)
	}

	// Get license challenge
	challenge, err := a.getLicenseChallenge(ctx, sessionID, pssh)
	if err != nil {
		return nil, fmt.Errorf("drmremote: get license challenge: %w", err)
	}

	// Send challenge to license server
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, licenseURL, bytes.NewReader(challenge))
	if err != nil {
		return nil, fmt.Errorf("drmremote: create license request: %w", err)
	}
	if authContext != "" {
		req.Header.Set("Authorization", authContext)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("drmremote: license request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("drmremote: license server returned %s", resp.Status)
	}

	licenseResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("drmremote: read license response: %w", err)
	}

	// Parse license response
	if err := a.parseLicense(ctx, sessionID, licenseResp); err != nil {
		return nil, fmt.Errorf("drmremote: parse license: %w", err)
	}

	// Get keys
	keys, err := a.getKeys(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("drmremote: get keys: %w", err)
	}

	// Convert to ContentKey
	out := make([]*slotsv1.ContentKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, &slotsv1.ContentKey{
			KeyId:       fmt.Sprintf("%x", k.ID),
			KeyMaterial: k.Key,
			Validity:    durationpb.New(time.Hour), // default validity
		})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("drmremote: no content keys in license response")
	}

	a.logger.Info("drmremote: acquired keys",
		"provider", providerID,
		"content", contentID,
		"system", system,
		"keys", len(out),
	)
	return out, nil
}

// Decrypt decrypts an encrypted track source into clear bytes using the
// content keys. Decryption happens locally using gowidevine's DecryptMP4Auto.
func (a *Adapter) Decrypt(ctx context.Context, source drm.Source, keys []*slotsv1.ContentKey) ([]byte, error) {
	if len(source.InlineBytes) == 0 {
		return nil, fmt.Errorf("drmremote: inline_bytes is required")
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("drmremote: at least one key is required")
	}

	// Convert ContentKey to gowidevine Key
	wvKeys := make([]*widevine.Key, 0, len(keys))
	for _, k := range keys {
		wvKeys = append(wvKeys, &widevine.Key{
			ID:  []byte(k.GetKeyId()),
			Key: k.GetKeyMaterial(),
		})
	}

	// Decrypt MP4 locally
	var buf bytes.Buffer
	if err := widevine.DecryptMP4Auto(bytes.NewReader(source.InlineBytes), wvKeys, &buf); err != nil {
		return nil, fmt.Errorf("drmremote: decrypt: %w", err)
	}

	a.logger.Debug("drmremote: decrypted", "bytes", buf.Len())
	return buf.Bytes(), nil
}

// openSession opens a CDM session on the remote service.
func (a *Adapter) openSession(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/%s/open", a.serviceURL, a.deviceName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if a.secretKey != "" {
		req.Header.Set("X-Secret-Key", a.secretKey)
	}

	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if apiResp.Status != 200 {
		return "", fmt.Errorf("api error: %s", apiResp.Message)
	}

	var data struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(apiResp.Data, &data); err != nil {
		return "", fmt.Errorf("decode data: %w", err)
	}
	return data.SessionID, nil
}

// getLicenseChallenge gets a license challenge from the remote service.
func (a *Adapter) getLicenseChallenge(ctx context.Context, sessionID string, pssh []byte) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/get_license_challenge/STREAMING", a.serviceURL, a.deviceName)
	payload := map[string]any{
		"session_id":   sessionID,
		"init_data":    base64.StdEncoding.EncodeToString(pssh),
		"privacy_mode": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if a.secretKey != "" {
		req.Header.Set("X-Secret-Key", a.secretKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if apiResp.Status != 200 {
		return nil, fmt.Errorf("api error: %s", apiResp.Message)
	}

	var data struct {
		ChallengeB64 string `json:"challenge_b64"`
	}
	if err := json.Unmarshal(apiResp.Data, &data); err != nil {
		return nil, fmt.Errorf("decode data: %w", err)
	}

	return base64.StdEncoding.DecodeString(data.ChallengeB64)
}

// parseLicense sends the license response to the remote service for parsing.
func (a *Adapter) parseLicense(ctx context.Context, sessionID string, licenseResp []byte) error {
	url := fmt.Sprintf("%s/%s/parse_license", a.serviceURL, a.deviceName)
	payload := map[string]any{
		"session_id":      sessionID,
		"license_message": base64.StdEncoding.EncodeToString(licenseResp),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if a.secretKey != "" {
		req.Header.Set("X-Secret-Key", a.secretKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if apiResp.Status != 200 {
		return fmt.Errorf("api error: %s", apiResp.Message)
	}
	return nil
}

// getKeys retrieves the content keys from the remote service.
func (a *Adapter) getKeys(ctx context.Context, sessionID string) ([]*widevine.Key, error) {
	url := fmt.Sprintf("%s/%s/get_keys/ALL", a.serviceURL, a.deviceName)
	payload := map[string]any{
		"session_id": sessionID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if a.secretKey != "" {
		req.Header.Set("X-Secret-Key", a.secretKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if apiResp.Status != 200 {
		return nil, fmt.Errorf("api error: %s", apiResp.Message)
	}

	var data struct {
		Keys []struct {
			KeyID string `json:"key_id"`
			Key   string `json:"key"`
			Type  string `json:"type"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(apiResp.Data, &data); err != nil {
		return nil, fmt.Errorf("decode data: %w", err)
	}

	keys := make([]*widevine.Key, 0, len(data.Keys))
	for _, k := range data.Keys {
		keyID, err := hex.DecodeString(k.KeyID)
		if err != nil {
			return nil, fmt.Errorf("decode key id: %w", err)
		}
		keyBytes, err := hex.DecodeString(k.Key)
		if err != nil {
			return nil, fmt.Errorf("decode key: %w", err)
		}
		keys = append(keys, &widevine.Key{
			ID:  keyID,
			Key: keyBytes,
		})
	}
	return keys, nil
}
