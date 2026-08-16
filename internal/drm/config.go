package drm

import (
	"fmt"
	"time"
)

// Config is the top-level DRM configuration. It wires key providers for each
// scheme into a shared Engine used by the convert pipeline.
type Config struct {
	// Enabled turns on server-side DRM decryption during conversion.
	Enabled bool `yaml:"enabled"`
	// Widevine configures the Widevine key provider (device dir + backend).
	Widevine WidevineConfig `yaml:"widevine"`
	// PlayReady configures the PlayReady key provider.
	PlayReady PlayReadyConfig `yaml:"playready"`
	// ClearKey configures static ClearKey material.
	ClearKey ClearKeyConfig `yaml:"clearkey"`
	// AES128 configures the HLS AES-128 key provider.
	AES128 AES128Config `yaml:"aes128"`
	// TTL is how long licensed keys stay cached in the vault. Defaults to 12h.
	TTL time.Duration `yaml:"ttl"`
}

// BuildEngine builds a decryption Engine from cfg, wiring each configured
// provider through the shared in-memory vault. Returns a no-op Engine when DRM
// is disabled, and an error when a configured provider cannot be built.
func BuildEngine(cfg Config) (*Engine, error) {
	if !cfg.Enabled {
		return NewEngineSet(nil), nil
	}
	store := NewMemoryVaultStore()
	providers := make(map[Scheme]KeyProvider)

	if cfg.Widevine.DeviceDir != "" || cfg.Widevine.WVD != "" {
		p, err := NewWidevine(cfg.Widevine)
		if err != nil {
			return nil, fmt.Errorf("drm: widevine: %w", err)
		}
		providers[SchemeWidevine] = NewVault(store, p, cfg.TTL)
	}
	if cfg.PlayReady.DeviceDir != "" {
		p, err := NewPlayReady(cfg.PlayReady)
		if err != nil {
			return nil, fmt.Errorf("drm: playready: %w", err)
		}
		providers[SchemePlayReady] = NewVault(store, p, cfg.TTL)
	}
	if len(cfg.ClearKey.Keys) > 0 || len(cfg.ClearKey.JWKSet) > 0 {
		p, err := NewClearKey(cfg.ClearKey)
		if err != nil {
			return nil, fmt.Errorf("drm: clearkey: %w", err)
		}
		providers[SchemeClearKey] = NewVault(store, p, cfg.TTL)
	}
	if cfg.AES128.Enabled {
		p, err := NewAES128(cfg.AES128)
		if err != nil {
			return nil, fmt.Errorf("drm: aes128: %w", err)
		}
		providers[SchemeAES128] = NewVault(store, p, cfg.TTL)
	}
	return NewEngineSet(providers), nil
}
