package drm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// DecodeWidevinePSSH parses a Widevine PSSH payload (the pssh box data) into
// its KID and content ID fields using diana's pure-Go protobuf decoder.
func DecodeWidevinePSSH(data []byte) (*widevinePSSH, error) {
	if len(data) == 0 {
		return nil, ErrPSSHNotFound
	}
	d, err := decodeWidevinePSSH(data)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// licenseClient performs HTTP license requests on behalf of providers.
type licenseClient struct {
	http *http.Client
}

// newLicenseClient returns a license client with sane timeouts.
func newLicenseClient() *licenseClient {
	return &licenseClient{http: &http.Client{Timeout: licenseTimeout}}
}

// post sends the challenge bytes to the license URL and returns the response body.
func (c *licenseClient) post(ctx context.Context, url string, headers http.Header, body []byte) ([]byte, error) {
	if url == "" {
		return nil, ErrNotConfigured
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("drm: license request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("drm: license request failed: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}
