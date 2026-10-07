package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/policy"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// StoreConfig describes a single store backend.
type StoreConfig struct {
	Backend string `yaml:"backend"`
	Path    string `yaml:"path,omitempty"`
}

// AccountConfig declares one login on the server its provider slot serves
// (IMPLEMENTATION.md §3: operator-declared accounts). The password is never
// written down — it is resolved from an environment variable named by
// password-env, and the provider session token that replaces it is stored
// sealed in the vault.
type AccountConfig struct {
	ID          string `yaml:"id"`
	Username    string `yaml:"username"`
	PasswordEnv string `yaml:"password-env"`
	// MaxConcurrentStreams is the declared ceiling for this account: the
	// operator's statement of what the upstream allows (PLAN.md §7.2). 0 or
	// absent leaves the account uncapped — the instance default policy
	// still applies to every member. Mirrors the linked-account record's
	// MaxConcurrentStreams.
	MaxConcurrentStreams uint32 `yaml:"max-concurrent-streams,omitempty"`
	// Policy overrides instance defaults for this account only. Keys absent
	// here inherit the instance policy; malformed values or unknown keys
	// refuse startup, never silently ignore. Validated against the policy
	// package's strict vocabulary.
	Policy map[string]string `yaml:"policy,omitempty"`
}

// SlotEntry is one declared slot instance within a kind list. The kind comes
// from which list it sits in (PLAN.md §3.1 fixes five kinds); adapter selects
// the implementation registered for it. Adding an instance of an existing
// adapter is a pure config change.
type SlotEntry struct {
	Adapter     string `yaml:"adapter"`
	ID          string `yaml:"id"`
	Transport   string `yaml:"transport"`
	Enabled     bool   `yaml:"enabled"`
	SyncCadence string `yaml:"sync-cadence"`
	// TokenEnv names the environment variable carrying a slot-level API
	// secret (e.g. the TMDB bearer token); the value never lives in config
	// (TECHNICAL-DECISIONS §1.27). Optional; only adapters that authenticate
	// instance-wide read it.
	TokenEnv string `yaml:"token-env"`
	// Server is the base URL of the single server this provider slot serves
	// (PLAN.md §3.1). Accounts of a slot are logins on that server — a slot
	// never spans servers. Catalogue and sink entries leave it empty.
	Server   string          `yaml:"server"`
	Accounts []AccountConfig `yaml:"accounts"`
	// Options is the per-adapter configuration bag. Each adapter reads only
	// the keys it declares; the shared SlotEntry stays free of adapter-specific
	// fields, so an adapter never sees another adapter's knobs (PLAN.md §6.4:
	// sinks are pluggable slots). Values are strings; an adapter parses them.
	// E.g. the disk sink reads Options["path"].
	Options map[string]string `yaml:"options"`
}

// SlotsConfig mirrors PLAN.md §3.1's taxonomy: the built-in reference slot,
// then one open-ended instance list per slot kind. Lists for kinds whose
// milestones have not landed stay empty; their shape is already fixed so a
// future milestone never rewrites this schema.
type SlotsConfig struct {
	Builtin struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"builtin"`
	Providers       []SlotEntry `yaml:"providers"`
	Catalogue       []SlotEntry `yaml:"catalogue"`
	Sinks           []SlotEntry `yaml:"sinks"`
	SubtitleSources []SlotEntry `yaml:"subtitle-sources"`
	Drm             []SlotEntry `yaml:"drm"`
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
		// DEKCache selects where unwrapped per-session data-encryption keys
		// live: "memory" (default) keeps them in process memory only;
		// "encrypted-store" persists them in the sessions store sealed with
		// the vault cipher — no plaintext key material reaches disk either
		// way (IMPLEMENTATION.md §1.3). With an ephemeral vault key,
		// encrypted-store entries are unreadable after a restart.
		DEKCache string `yaml:"dek-cache"`
	} `yaml:"auth"`
	Stores struct {
		Caches        StoreConfig `yaml:"caches"`
		Vault         StoreConfig `yaml:"vault"`
		VaultKey      string      `yaml:"vault-key"`
		WatchHistory  StoreConfig `yaml:"watch-history"`
		Jobs          StoreConfig `yaml:"jobs"`
		Sessions      StoreConfig `yaml:"sessions"`
		Users         StoreConfig `yaml:"users"`
		SourceCache   StoreConfig `yaml:"source-cache"`
		MetadataCache StoreConfig `yaml:"metadata-cache"`
	} `yaml:"stores"`
	// Policy is the instance-wide usage policy — a limit-type → value map
	// (PLAN.md §7.2). Absent keys inherit the shipped defaults; unknown
	// keys and malformed values fail startup. The delivery engine stamps it
	// on every job's recorded DeliveryContext.
	Policy map[string]string `yaml:"policy,omitempty"`
	Slots  SlotsConfig       `yaml:"slots"`
	// Enrichment tunes the background metadata pipeline. Absent keys fall
	// back to the defaults the enrichment package declares.
	Enrichment EnrichmentConfig `yaml:"enrichment"`
	// Delivery tunes session admission, session liveness and the
	// cap-change behaviour (TECHNICAL-DECISIONS.md §1.14). Absent keys
	// fall back to the shipped defaults.
	Delivery DeliveryConfig `yaml:"delivery"`
	// Library tunes matching (PLAN.md §5.3). Absent keys fall back to the
	// shipped defaults.
	Library LibraryConfig `yaml:"library"`
}

// DeliveryConfig carries the delivery engine's operator knobs.
type DeliveryConfig struct {
	// OnCapChange selects what lowering an account's concurrent-stream cap
	// does to the sessions already running on it: "new-sessions-only"
	// (default) lets running streams finish and applies the new cap from
	// the next session; "enforce-now" ends the excess sessions immediately,
	// oldest first. An account may override this with its own
	// cap_change_policy.
	OnCapChange string `yaml:"on-cap-change"`
	// SessionTTL is the zombie cap: a session with no liveness proof ends
	// after this (PLAN.md §9.1). Go duration string; empty means the
	// shipped default.
	SessionTTL string `yaml:"session-ttl"`
	// Heartbeat is the play-session liveness contract (PLAN.md §9.1).
	Heartbeat HeartbeatConfig `yaml:"heartbeat"`
}

// HeartbeatConfig carries the play-session liveness knobs. The interval is
// published to clients via GetInstanceInfo, so clients never hardcode it.
type HeartbeatConfig struct {
	// Interval is how often a play session's frontend must prove liveness.
	// Go duration string; empty means the shipped default.
	Interval string `yaml:"interval"`
	// Grace is the server-side slack before a missed heartbeat ends the
	// session. Go duration string; empty means the shipped default.
	Grace string `yaml:"grace"`
}

// LibraryConfig carries matching knobs (PLAN.md §5.3).
type LibraryConfig struct {
	// Articles are the leading articles dropped when titles are normalized
	// for matching. Absent means the shipped default list; an explicit
	// empty list drops no articles. Set before the first sync: the item
	// registry's normalized-title index is built with the list in effect
	// at write time, so changing it on a populated instance requires
	// rebuilding the identity store.
	Articles []string `yaml:"articles"`
}

// EnrichmentConfig carries the enrichment pipeline's operator knobs.
type EnrichmentConfig struct {
	// DrainCadence overrides how often the drain worker checks the queue;
	// empty means the package default (TECHNICAL-DECISIONS.md §1.29).
	DrainCadence string `yaml:"drain-cadence"`
}

// Stores holds the instantiated store backends for each storage class
// (PLAN.md §2.4). VaultAEAD is the single instance key resolved from
// stores.vault-key; the vault, the sealed users store and an encrypted
// data-key cache all derive from it.
type Stores struct {
	Cache         store.Store
	Vault         store.Store
	WatchHistory  store.Store
	Jobs          store.Store
	Sessions      store.Store
	Users         store.Store
	SourceCache   store.Store
	MetadataCache store.Store
	VaultAEAD     cipher.AEAD
}

func Default() *Config {
	c := &Config{}
	c.Core.API.Bind = "127.0.0.1:8443"
	c.Auth.Methods = []string{"password"}
	c.Auth.TokenTTL = "168h"
	c.Auth.DEKCache = "memory"
	c.Stores.Caches = StoreConfig{Backend: "in-memory"}
	c.Stores.Vault = StoreConfig{Backend: "in-memory"}
	c.Stores.VaultKey = "generated"
	c.Stores.WatchHistory = StoreConfig{Backend: "in-memory"}
	c.Stores.Jobs = StoreConfig{Backend: "in-memory"}
	c.Stores.Sessions = StoreConfig{Backend: "in-memory"}
	c.Stores.Users = StoreConfig{Backend: "in-memory"}
	// Dev default stays lightweight like every other class; the example
	// config recommends local-file for instances that want restart-fast
	// catalogues instead of a rebuild-from-provider.
	c.Stores.SourceCache = StoreConfig{Backend: "in-memory"}
	// Same tradeoff as the source cache: enrichment (M3) rebuilds the
	// metadata cache from catalogue slots, so dev loses nothing by staying
	// in-memory.
	c.Stores.MetadataCache = StoreConfig{Backend: "in-memory"}
	// The built-in reference slot is part of every instance; kind lists start
	// empty and grow through operator config.
	c.Slots.Builtin.Enabled = true
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
	if err := validateSlots(c.Slots); err != nil {
		return nil, fmt.Errorf("config: slots: %w", err)
	}
	if err := validatePolicies(c); err != nil {
		return nil, err
	}
	// Timing and matching knobs fail the load the same way policy keys do:
	// a mis-typed value refuses boot rather than running on a silent
	// fallback.
	if _, err := ParseDeliveryTiming(c.Delivery); err != nil {
		return nil, err
	}
	if err := ValidateArticles(c.Library.Articles); err != nil {
		return nil, err
	}
	return c, nil
}

// validatePolicies fails the load when the instance policy block or any
// account policy block fails the strict policy-vocabulary check. Failing
// here — at boot, once, with the offending key named — is what a mis-typed
// limit must produce; never starting confined by nothing.
func validatePolicies(c *Config) error {
	if _, err := policy.ParseInstance(c.Policy); err != nil {
		return err
	}
	for _, list := range [][]SlotEntry{c.Slots.Providers, c.Slots.Catalogue, c.Slots.Sinks} {
		for _, e := range list {
			for _, a := range e.Accounts {
				if _, err := policy.ParseOverlay(a.Policy); err != nil {
					return fmt.Errorf("account %q in slot %q: %w", a.ID, e.ID, err)
				}
			}
		}
	}
	return nil
}

// validateSlots enforces the invariants the slot taxonomy implies (PLAN.md
// §3.1): instance IDs are unique across every kind, each entry names its
// adapter, and v1 speaks exactly one transport — a subprocess entry must fail
// loudly rather than be silently ignored. Provider slots additionally must
// declare the one server they serve, and every account inside them needs an id.
func validateSlots(slots SlotsConfig) error {
	seen := map[string]string{}
	for _, list := range []struct {
		kind    string
		entries []SlotEntry
	}{
		{"providers", slots.Providers},
		{"catalogue", slots.Catalogue},
		{"sinks", slots.Sinks},
		{"subtitle-sources", slots.SubtitleSources},
		{"drm", slots.Drm},
	} {
		for _, e := range list.entries {
			if e.ID == "" {
				return fmt.Errorf("%s entry: id is required", list.kind)
			}
			if prev := seen[e.ID]; prev != "" {
				return fmt.Errorf("duplicate slot id %q (also in %s)", e.ID, prev)
			}
			seen[e.ID] = list.kind
			if e.Adapter == "" {
				return fmt.Errorf("slot %q: adapter is required", e.ID)
			}
			switch e.Transport {
			case "", "in-process":
				// v1 ships in-process only; empty means the default.
			default:
				return fmt.Errorf("slot %q: unsupported transport %q (v1 supports \"in-process\")", e.ID, e.Transport)
			}
			if list.kind == "providers" {
				if e.Server == "" {
					return fmt.Errorf("slot %q: a provider slot must declare the server it serves", e.ID)
				}
				for _, a := range e.Accounts {
					if a.ID == "" {
						return fmt.Errorf("slot %q: account entry missing id", e.ID)
					}
				}
			}
			if err := validateSlotOptions(e); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateSlotOptions fails the load when a slot's pacing knobs in the
// options bag are mis-typed: a silent fallback would run the instance with
// no aggregate bound, the opposite of what the operator asked for (§2.5;
// mirrors the strict policy-key validation in validatePolicies).
func validateSlotOptions(e SlotEntry) error {
	for k, v := range e.Options {
		switch k {
		case policy.KeyPacingRequestsPerSec:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f <= 0 {
				return fmt.Errorf("slot %q: options %q must be a positive number, got %q", e.ID, k, v)
			}
		case policy.KeyPacingMaxPulls:
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("slot %q: options %q must be a positive integer, got %q", e.ID, k, v)
			}
		case policy.KeyPacingDelay:
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return fmt.Errorf("slot %q: options %q must be a positive duration, got %q", e.ID, k, v)
			}
		case policy.KeyPacingRetries:
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("slot %q: options %q must be a non-negative integer, got %q", e.ID, k, v)
			}
		}
	}
	return nil
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

	s.SourceCache, err = buildStore(ctx, cfg.Stores.SourceCache, "data/source-cache.db")
	if err != nil {
		return s, fmt.Errorf("stores.source-cache: %w", err)
	}

	s.MetadataCache, err = buildStore(ctx, cfg.Stores.MetadataCache, "data/metadata-cache.db")
	if err != nil {
		return s, fmt.Errorf("stores.metadata-cache: %w", err)
	}

	// The instance key is resolved exactly once and shared by everything that
	// must be sealed at rest: the vault store, the sealed login store, and an
	// encrypted data-key cache. Resolving it per consumer would, with a
	// generated key, hand each of them a different key — and sealed records
	// would become unreadable in-process as well.
	aead, err := loadOrGenerateVaultKey(cfg.Stores.VaultKey, logger)
	if err != nil {
		return s, fmt.Errorf("stores.vault key: %w", err)
	}
	s.VaultAEAD = aead

	// Login records carry password hashes and wrapped keys. Seal them with the
	// instance key regardless of backend so the guarantee (records are never
	// plaintext at rest) does not depend on a config dropdown.
	s.Users, err = store.NewSealed(s.Users, aead, logger)
	if err != nil {
		return s, fmt.Errorf("stores.users: %w", err)
	}

	// Vault requires an AEAD cipher — the single one resolved above is it.
	switch cfg.Stores.Vault.Backend {
	case "in-memory":
		s.Vault = store.NewInMemory()
	case "local-file":
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

	auditDurability(cfg, logger)
	return s, nil
}

// CloseStores releases every store backend in the set; the first error wins,
// all stores get closed regardless.
func CloseStores(s Stores) error {
	var firstErr error
	for _, c := range []store.Store{s.Cache, s.Vault, s.WatchHistory, s.Jobs, s.Sessions, s.Users, s.SourceCache, s.MetadataCache} {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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

// loadOrGenerateVaultKey returns an AEAD cipher for the instance key. If the
// config value is "generated", a random 32-byte key is created — fine within
// one process, useless across a restart, which auditDurability names.
// Otherwise the value is hex-decoded as a 32-byte key.
func loadOrGenerateVaultKey(val string, _ *slog.Logger) (cipher.AEAD, error) {
	var key []byte

	switch val {
	case "generated", "":
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate vault key: %w", err)
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

// auditDurability is the one voice on store durability at boot. The storage
// classification says what may be lost and what must not; rather than trusting
// the operator to remember, boot inspects the effective configuration and
// names, per affected class, exactly what a restart takes away. It never
// refuses to boot: a throwaway instance is a legitimate configuration, and
// it should be able to say so honestly out loud.
func auditDurability(cfg *Config, logger *slog.Logger) {
	if logger == nil {
		return
	}

	// A generated key never matches itself across a restart. Whatever is
	// sealed on disk — account sessions, user records, an encrypted data-key
	// cache — is irrecoverable afterwards, even though the files remain.
	// In-memory classes lose their contents with the process either way.
	if cfg.Stores.VaultKey == "" || cfg.Stores.VaultKey == "generated" {
		logger.Warn("stores: no instance key configured (stores.vault-key is generated): every sealed store becomes unreadable after a restart. Pin a 64-hex-character key to keep logins, account sessions and watch history.")
	}

	var lost []string
	if cfg.Stores.Users.Backend == "in-memory" {
		lost = append(lost, "logins (users)")
	}
	if cfg.Stores.Vault.Backend == "in-memory" {
		lost = append(lost, "account sessions (vault)")
	}
	if cfg.Stores.WatchHistory.Backend == "in-memory" {
		lost = append(lost, "watch history")
	}
	if len(lost) > 0 {
		logger.Warn("stores: must-not-lose classes are in-memory — a restart loses them", "lost", lost)
	}
	if cfg.Stores.Jobs.Backend == "in-memory" {
		logger.Warn("stores: jobs are in-memory — a restart restarts in-flight work instead of continuing it")
	}
}

// ParseCapChangeDefault resolves the operator's delivery.on-cap-change
// setting into the accounts vocabulary. An empty value resolves to the
// shipped default, "new-sessions-only": a lowered cap never kills a running
// session. Unknown values are a startup failure, never a silent fallback.
func ParseCapChangeDefault(raw string) (accounts.CapChangePolicy, error) {
	switch strings.TrimSpace(raw) {
	case "", "new-sessions-only":
		return accounts.CapChangePolicyNewSessionsOnly, nil
	case "enforce-now":
		return accounts.CapChangePolicyEnforceNow, nil
	default:
		return accounts.CapChangePolicyDefault, fmt.Errorf("delivery: unknown on-cap-change %q (want \"new-sessions-only\" or \"enforce-now\")", raw)
	}
}

// Shipped delivery-timing defaults (TECHNICAL-DECISIONS.md §1.14). The
// values' single home is here; every consumer resolves through
// ParseDeliveryTiming rather than restating them.
const (
	DefaultSessionTTL        = 24 * time.Hour
	DefaultHeartbeatInterval = 30 * time.Second
	DefaultHeartbeatGrace    = 90 * time.Second
)

// DeliveryTiming is the resolved delivery-engine timing: the zombie cap and
// the play-session liveness contract (PLAN.md §9.1).
type DeliveryTiming struct {
	SessionTTL        time.Duration
	HeartbeatInterval time.Duration
	HeartbeatGrace    time.Duration
}

// ParseDeliveryTiming resolves the delivery timing knobs: an absent key
// falls back to the shipped default; a malformed or non-positive duration
// is an error naming the key, never a silent fallback.
func ParseDeliveryTiming(c DeliveryConfig) (DeliveryTiming, error) {
	t := DeliveryTiming{
		SessionTTL:        DefaultSessionTTL,
		HeartbeatInterval: DefaultHeartbeatInterval,
		HeartbeatGrace:    DefaultHeartbeatGrace,
	}
	var err error
	if t.SessionTTL, err = durationOr(c.SessionTTL, t.SessionTTL, "delivery.session-ttl"); err != nil {
		return t, err
	}
	if t.HeartbeatInterval, err = durationOr(c.Heartbeat.Interval, t.HeartbeatInterval, "delivery.heartbeat.interval"); err != nil {
		return t, err
	}
	if t.HeartbeatGrace, err = durationOr(c.Heartbeat.Grace, t.HeartbeatGrace, "delivery.heartbeat.grace"); err != nil {
		return t, err
	}
	return t, nil
}

// durationOr parses a config duration, returning fallback when the value is
// absent and an error naming the key when it is broken.
func durationOr(raw string, fallback time.Duration, key string) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("config: %s %q is not a positive duration", key, raw)
	}
	return d, nil
}

// ValidateArticles rejects article entries that can never fire: title
// normalization compares lowercase whitespace-separated tokens, so an entry
// that is empty, uppercased, or contains whitespace is a typo the operator
// should hear about at startup, not a silent no-op.
func ValidateArticles(articles []string) error {
	for _, a := range articles {
		if a == "" || strings.ContainsAny(a, " \t\n") || a != strings.ToLower(a) {
			return fmt.Errorf("config: library.articles entry %q is not a single lowercase word", a)
		}
	}
	return nil
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

// BuildAuth creates the auth-layer stores from the given backend stores and
// the configured DEK-cache mode ("memory" or "encrypted-store"; empty means
// memory). The encrypted-store mode seals every entry with aead, which must
// then be non-nil.
func BuildAuth(users, sessions store.Store, dekCacheMode string, aead cipher.AEAD) (auth.UserStore, auth.TokenStore, auth.DEKCache, error) {
	var deks auth.DEKCache
	switch dekCacheMode {
	case "", "memory":
		deks = auth.NewMemoryDEKCache()
	case "encrypted-store":
		sealed, err := auth.NewSealedDEKCache(sessions, aead)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("auth: dek-cache: %w", err)
		}
		deks = sealed
	default:
		return nil, nil, nil, fmt.Errorf("auth: unknown dek-cache mode %q (want \"memory\" or \"encrypted-store\")", dekCacheMode)
	}
	return auth.NewStoreUserStore(users), auth.NewStoreTokenStore(sessions), deks, nil
}

// BuildAuthenticator creates a CompositeAuthenticator from the configured methods.
func BuildAuthenticator(methods []string, userStore auth.UserStore) (*auth.CompositeAuthenticator, error) {
	return auth.NewAuthenticators(methods, userStore)
}

// BuildSession creates a SessionHandler from the given stores and TTL.
func BuildSession(tokens auth.TokenStore, deks auth.DEKCache, ttl time.Duration) auth.Session {
	return auth.NewSessionHandler(tokens, deks, ttl)
}
