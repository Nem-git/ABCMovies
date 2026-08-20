package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/nem-git/abcmovies/core/internal/store"
)

type Config struct {
	Core struct {
		API struct {
			Bind string `yaml:"bind"`
		} `yaml:"api"`
	} `yaml:"core"`
	Auth struct {
		TokenTTL string `yaml:"token-ttl"` // e.g. "168h" for 7 days
	} `yaml:"auth"`
	Stores struct {
		Caches       string `yaml:"caches"`
		Vault        string `yaml:"vault"`
		VaultKey     string `yaml:"vault-key"`
		WatchHistory string `yaml:"watch-history"`
		Jobs         string `yaml:"jobs"`
	} `yaml:"stores"`
}

// Stores holds the instantiated store backends for each storage class
// (PLAN.md §2.4).
type Stores struct {
	Cache        store.Store
	Vault        store.Store
	WatchHistory store.Store
	Jobs         store.Store
}

func Default() *Config {
	c := &Config{}
	c.Core.API.Bind = "127.0.0.1:8443"
	c.Auth.TokenTTL = "168h" // 7 days
	c.Stores.Caches = "in-memory"
	c.Stores.Vault = "in-memory"
	c.Stores.VaultKey = "generated"
	c.Stores.WatchHistory = "in-memory"
	c.Stores.Jobs = "in-memory"
	return c
}

func Load(path string) (*Config, error) {
	c := Default()
	if path == "" {
		return c, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return c, nil
}

// BuildStores instantiates store backends from the config. "in-memory" creates
// an InMemory store; "local-file" creates a SQLite store (for caches, jobs,
// watch-history) or a Vault (for vault). The vault-key config controls the
// AEAD cipher: "generated" creates a random key at startup (dev mode);
// otherwise the value is treated as a hex-encoded 32-byte key.
func BuildStores(ctx context.Context, cfg *Config, logger *slog.Logger) (Stores, error) {
	var s Stores
	var err error

	s.Cache, err = buildStore(ctx, cfg.Stores.Caches, "")
	if err != nil {
		return s, fmt.Errorf("stores.caches: %w", err)
	}

	s.WatchHistory, err = buildStore(ctx, cfg.Stores.WatchHistory, "")
	if err != nil {
		return s, fmt.Errorf("stores.watch-history: %w", err)
	}

	s.Jobs, err = buildStore(ctx, cfg.Stores.Jobs, "")
	if err != nil {
		return s, fmt.Errorf("stores.jobs: %w", err)
	}

	// Vault requires an AEAD cipher.
	switch cfg.Stores.Vault {
	case "in-memory":
		s.Vault = store.NewInMemory()
	case "local-file":
		aead, vaultErr := loadOrGenerateVaultKey(cfg.Stores.VaultKey, logger)
		if vaultErr != nil {
			return s, fmt.Errorf("stores.vault key: %w", vaultErr)
		}
		s.Vault, err = store.NewVault(ctx, "vault.db", aead)
		if err != nil {
			return s, fmt.Errorf("stores.vault: %w", err)
		}
	default:
		return s, fmt.Errorf("stores.vault: unknown backend %q", cfg.Stores.Vault)
	}

	return s, nil
}

func buildStore(_ context.Context, backend, path string) (store.Store, error) {
	switch backend {
	case "in-memory":
		return store.NewInMemory(), nil
	case "local-file":
		if path == "" {
			path = "store.db"
		}
		return store.NewSQLite(context.Background(), path)
	default:
		return nil, fmt.Errorf("unknown backend %q", backend)
	}
}

// loadOrGenerateVaultKey returns an AEAD cipher for the vault. If the config
// value is "generated", a random 32-byte key is created and a warning is
// logged (data will not survive restart). Otherwise the value is hex-decoded
// as a 32-byte key.
func loadOrGenerateVaultKey(val string, logger *slog.Logger) (cipher.AEAD, error) {
	var key []byte

	switch val {
	case "generated", "":
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate vault key: %w", err)
		}
		if logger != nil {
			logger.Warn("generated ephemeral vault key — data will not survive restart")
		}
	default:
		// Treat as hex-encoded 32-byte key.
		if len(val) != 64 {
			return nil, fmt.Errorf("vault-key must be 64 hex characters (32 bytes), got %d", len(val))
		}
		key = make([]byte, 32)
		for i := 0; i < 32; i++ {
			var b byte
			_, err := fmt.Sscanf(val[i*2:i*2+2], "%02x", &b)
			if err != nil {
				return nil, fmt.Errorf("vault-key: invalid hex at position %d: %w", i, err)
			}
			key[i] = b
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return aead, nil
}

// ParseTokenTTL parses the token TTL from the config string.
// Returns the default (7 days) if the string is empty or invalid.
func ParseTokenTTL(val string) time.Duration {
	const defaultTTL = 168 * time.Hour // 7 days
	if val == "" {
		return defaultTTL
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultTTL
	}
	return d
}
