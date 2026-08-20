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

	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// StoreConfig describes a single store backend.
type StoreConfig struct {
	Backend string `yaml:"backend"`
	Path    string `yaml:"path,omitempty"`
}

type Config struct {
	Core struct {
		API struct {
			Bind string `yaml:"bind"`
		} `yaml:"api"`
	} `yaml:"core"`
	Auth struct {
		Methods  []string `yaml:"methods"`
		TokenTTL string   `yaml:"token-ttl"`
	} `yaml:"auth"`
	Stores struct {
		Caches       StoreConfig `yaml:"caches"`
		Vault        StoreConfig `yaml:"vault"`
		VaultKey     string      `yaml:"vault-key"`
		WatchHistory StoreConfig `yaml:"watch-history"`
		Jobs         StoreConfig `yaml:"jobs"`
		Sessions     StoreConfig `yaml:"sessions"`
		Users        StoreConfig `yaml:"users"`
	} `yaml:"stores"`
}

// Stores holds the instantiated store backends for each storage class
// (PLAN.md §2.4).
type Stores struct {
	Cache        store.Store
	Vault        store.Store
	WatchHistory store.Store
	Jobs         store.Store
	Sessions     store.Store
	Users        store.Store
}

func Default() *Config {
	c := &Config{}
	c.Core.API.Bind = "127.0.0.1:8443"
	c.Auth.Methods = []string{"password"}
	c.Auth.TokenTTL = "168h"
	c.Stores.Caches = StoreConfig{Backend: "in-memory"}
	c.Stores.Vault = StoreConfig{Backend: "in-memory"}
	c.Stores.VaultKey = "generated"
	c.Stores.WatchHistory = StoreConfig{Backend: "in-memory"}
	c.Stores.Jobs = StoreConfig{Backend: "in-memory"}
	c.Stores.Sessions = StoreConfig{Backend: "in-memory"}
	c.Stores.Users = StoreConfig{Backend: "in-memory"}
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

// BuildStores instantiates store backends from the config. Each store class
// gets a default file path to prevent collisions when multiple stores use
// "local-file" (PLAN.md §2.4).
func BuildStores(ctx context.Context, cfg *Config, logger *slog.Logger) (Stores, error) {
	var s Stores
	var err error

	s.Cache, err = buildStore(ctx, cfg.Stores.Caches, "data/caches.db")
	if err != nil {
		return s, fmt.Errorf("stores.caches: %w", err)
	}

	s.WatchHistory, err = buildStore(ctx, cfg.Stores.WatchHistory, "data/watch-history.db")
	if err != nil {
		return s, fmt.Errorf("stores.watch-history: %w", err)
	}
	// WatchHistory is a per-user encrypted blob (PLAN.md §2.4, IMPLEMENTATION.md
	// §1.3). Wrap with UserBlobStore so values are encrypted with the caller's
	// DEK from the request context.
	s.WatchHistory = store.NewUserBlobStore(s.WatchHistory)

	s.Jobs, err = buildStore(ctx, cfg.Stores.Jobs, "data/jobs.db")
	if err != nil {
		return s, fmt.Errorf("stores.jobs: %w", err)
	}

	s.Sessions, err = buildStore(ctx, cfg.Stores.Sessions, "data/sessions.db")
	if err != nil {
		return s, fmt.Errorf("stores.sessions: %w", err)
	}

	s.Users, err = buildStore(ctx, cfg.Stores.Users, "data/users.db")
	if err != nil {
		return s, fmt.Errorf("stores.users: %w", err)
	}

	// Vault requires an AEAD cipher.
	switch cfg.Stores.Vault.Backend {
	case "in-memory":
		s.Vault = store.NewInMemory()
	case "local-file":
		aead, vaultErr := loadOrGenerateVaultKey(cfg.Stores.VaultKey, logger)
		if vaultErr != nil {
			return s, fmt.Errorf("stores.vault key: %w", vaultErr)
		}
		vaultPath := cfg.Stores.Vault.Path
		if vaultPath == "" {
			vaultPath = "data/vault.db"
		}
		s.Vault, err = store.NewVault(ctx, vaultPath, aead)
		if err != nil {
			return s, fmt.Errorf("stores.vault: %w", err)
		}
	default:
		return s, fmt.Errorf("stores.vault: unknown backend %q", cfg.Stores.Vault.Backend)
	}

	return s, nil
}

func buildStore(_ context.Context, cfg StoreConfig, defaultPath string) (store.Store, error) {
	switch cfg.Backend {
	case "in-memory":
		return store.NewInMemory(), nil
	case "local-file":
		path := cfg.Path
		if path == "" {
			path = defaultPath
		}
		return store.NewSQLite(context.Background(), path)
	default:
		return nil, fmt.Errorf("unknown backend %q", cfg.Backend)
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

// BuildAuth creates the auth-layer stores from the given backend stores.
func BuildAuth(users, sessions store.Store) (auth.UserStore, auth.TokenStore, auth.DEKCache) {
	return auth.NewStoreUserStore(users),
		auth.NewStoreTokenStore(sessions),
		auth.NewStoreDEKCache(sessions)
}

// BuildAuthenticator creates a CompositeAuthenticator from the configured methods.
func BuildAuthenticator(methods []string, userStore auth.UserStore) (*auth.CompositeAuthenticator, error) {
	return auth.NewAuthenticators(methods, userStore)
}

// BuildSession creates a SessionHandler from the given stores and TTL.
func BuildSession(tokens auth.TokenStore, deks auth.DEKCache, ttl time.Duration) auth.Session {
	return auth.NewSessionHandler(tokens, deks, ttl)
}
