package drm

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ClearKeyConfig holds static ClearKey material. Keys may be supplied as a
// JSON Web Key Set (JWK) or as a plain KID -> key mapping.
type ClearKeyConfig struct {
	// JWKSet is an optional JSON Web Key Set (RFC 7517) with oct keys.
	JWKSet []byte
	// Keys maps KID (hex or base64url) to key (hex or base64url).
	Keys map[string]string
}

// ClearKeyProvider serves static content keys without any license request.
type ClearKeyProvider struct {
	keys map[KID]CEK
}

// NewClearKey builds a ClearKey provider from cfg.
func NewClearKey(cfg ClearKeyConfig) (*ClearKeyProvider, error) {
	keys := make(map[KID]CEK)
	if len(cfg.JWKSet) > 0 {
		kset, err := parseJWKSet(cfg.JWKSet)
		if err != nil {
			return nil, err
		}
		for kid, key := range kset {
			keys[kid] = key
		}
	}
	for kid, key := range cfg.Keys {
		k, err := decodeKeyID(kid)
		if err != nil {
			return nil, fmt.Errorf("drm: clearkey kid %q: %w", kid, err)
		}
		ck, err := decodeKey(key)
		if err != nil {
			return nil, fmt.Errorf("drm: clearkey key %q: %w", key, err)
		}
		keys[k] = ck
	}
	return &ClearKeyProvider{keys: keys}, nil
}

// Scheme implements KeyProvider.
func (p *ClearKeyProvider) Scheme() Scheme {
	return SchemeClearKey
}

// GetKeys returns the static keys matching the requested KIDs.
func (p *ClearKeyProvider) GetKeys(_ context.Context, req KeyRequest) (map[KID]CEK, error) {
	out := make(map[KID]CEK)
	for _, kid := range req.KIDs {
		key, ok := p.keys[kid]
		if !ok {
			continue
		}
		out[kid] = key
	}
	if len(out) == 0 {
		return nil, ErrCEKNotFound
	}
	return out, nil
}

// parseJWKSet decodes a JWK set into a KID -> key map. KIDs come from the
// "kid" header (hex/base64url) and keys from the base64url "k" parameter.
func parseJWKSet(data []byte) (map[KID]CEK, error) {
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			KID string `json:"kid"`
			K   string `json:"k"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("drm: parsing JWK set: %w", err)
	}
	out := make(map[KID]CEK)
	for _, jk := range set.Keys {
		if jk.Kty != "" && jk.Kty != "oct" {
			continue
		}
		kid, err := decodeKeyID(jk.KID)
		if err != nil {
			return nil, err
		}
		key, err := decodeKey(jk.K)
		if err != nil {
			return nil, err
		}
		out[kid] = key
	}
	return out, nil
}

// decodeKeyID decodes a KID from hex or base64url into a 16-byte key ID.
func decodeKeyID(s string) (KID, error) {
	var kid KID
	if s == "" {
		return kid, ErrKIDNotFound
	}
	b, err := decodeHexOrB64(s)
	if err != nil {
		return kid, err
	}
	if len(b) != 16 {
		return kid, fmt.Errorf("drm: key id must be 16 bytes, got %d", len(b))
	}
	copy(kid[:], b)
	return kid, nil
}

// decodeKey decodes a content key from hex or base64url into bytes.
func decodeKey(s string) (CEK, error) {
	if s == "" {
		return nil, ErrCEKNotFound
	}
	b, err := decodeHexOrB64(s)
	if err != nil {
		return nil, err
	}
	return CEK(b), nil
}

// decodeHexOrB64 decodes a string as hex, falling back to base64url.
func decodeHexOrB64(s string) ([]byte, error) {
	if b, err := hex.DecodeString(s); err == nil {
		return b, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("drm: not valid hex or base64url")
	}
	return b, nil
}
