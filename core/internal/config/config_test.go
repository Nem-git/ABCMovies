package config_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func TestDefault(t *testing.T) {
	c := config.Default()
	if c.Core.API.Bind != "127.0.0.1:8443" {
		t.Fatalf("Bind = %q, want %q", c.Core.API.Bind, "127.0.0.1:8443")
	}
	if len(c.Auth.Methods) != 1 || c.Auth.Methods[0] != "password" {
		t.Fatalf("Methods = %v, want [password]", c.Auth.Methods)
	}
	if c.Auth.TokenTTL != "168h" {
		t.Fatalf("TokenTTL = %q, want %q", c.Auth.TokenTTL, "168h")
	}
	if c.Stores.Caches.Backend != "in-memory" {
		t.Fatalf("Caches backend = %q, want %q", c.Stores.Caches.Backend, "in-memory")
	}
	if c.Stores.Vault.Backend != "in-memory" {
		t.Fatalf("Vault backend = %q, want %q", c.Stores.Vault.Backend, "in-memory")
	}
	if c.Stores.WatchHistory.Backend != "in-memory" {
		t.Fatalf("WatchHistory backend = %q, want %q", c.Stores.WatchHistory.Backend, "in-memory")
	}
	if c.Stores.Jobs.Backend != "in-memory" {
		t.Fatalf("Jobs backend = %q, want %q", c.Stores.Jobs.Backend, "in-memory")
	}
	if c.Stores.Sessions.Backend != "in-memory" {
		t.Fatalf("Sessions backend = %q, want %q", c.Stores.Sessions.Backend, "in-memory")
	}
	if c.Stores.Users.Backend != "in-memory" {
		t.Fatalf("Users backend = %q, want %q", c.Stores.Users.Backend, "in-memory")
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	c, err := config.Load("")
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if c.Core.API.Bind != "127.0.0.1:8443" {
		t.Fatalf("expected default bind, got %q", c.Core.API.Bind)
	}
}

func TestLoad_NonexistentPath(t *testing.T) {
	c, err := config.Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load nonexistent: %v", err)
	}
	if c.Core.API.Bind != "127.0.0.1:8443" {
		t.Fatalf("expected default bind, got %q", c.Core.API.Bind)
	}
}

func TestLoad_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := []byte(`
core:
  api:
    bind: "0.0.0.0:9090"
auth:
  methods: ["password"]
  token-ttl: "24h"
stores:
  caches:
    backend: in-memory
`)
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Core.API.Bind != "0.0.0.0:9090" {
		t.Fatalf("Bind = %q, want %q", c.Core.API.Bind, "0.0.0.0:9090")
	}
	if c.Auth.TokenTTL != "24h" {
		t.Fatalf("TokenTTL = %q, want %q", c.Auth.TokenTTL, "24h")
	}
}

func TestLoad_InvalidInstancePolicyFails(t *testing.T) {
	path := writeConfig(t, `
core:
  api:
    bind: "127.0.0.1:8443"
policy:
  concurrentStream: "3"
`)
	if _, err := config.Load(path); err == nil {
		t.Fatal("unknown policy key: want startup error")
	}
}

func TestLoad_InvalidAccountPolicyFails(t *testing.T) {
	path := writeConfig(t, `
slots:
  providers:
    - id: primary
      adapter: jellyfin
      enabled: true
      server: "http://jf.local"
      accounts:
        - id: home
          username: bob
          password-env: JF_PASSWORD
          max-concurrent-streams: 2
          policy:
            traffic: "high"
`)
	if _, err := config.Load(path); err == nil {
		t.Fatal("unknown account policy key: want startup error")
	}
}

func TestLoad_ValidPolicyAppliesDefaultsUnderneath(t *testing.T) {
	path := writeConfig(t, `
policy:
  concurrentStreams: "5"
slots:
  providers:
    - id: primary
      adapter: jellyfin
      enabled: true
      server: "http://jf.local"
      accounts:
        - id: home
          username: bob
          password-env: JF_PASSWORD
          max-concurrent-streams: 2
`)
	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := c.Policy["concurrentStreams"]; got != "5" {
		t.Fatalf("policy.concurrentStreams = %q, want 5", got)
	}
	alice := c.Slots.Providers[0].Accounts[0]
	if alice.MaxConcurrentStreams != 2 {
		t.Fatalf("max-concurrent-streams = %d, want 2", alice.MaxConcurrentStreams)
	}
}

func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestParseTokenTTL_Default(t *testing.T) {
	got := config.ParseTokenTTL("")
	want := 168 * time.Hour
	if got != want {
		t.Fatalf("ParseTokenTTL(\"\") = %v, want %v", got, want)
	}
}

func TestParseTokenTTL_Valid(t *testing.T) {
	got := config.ParseTokenTTL("24h")
	want := 24 * time.Hour
	if got != want {
		t.Fatalf("ParseTokenTTL(\"24h\") = %v, want %v", got, want)
	}
}

func TestParseTokenTTL_Invalid(t *testing.T) {
	got := config.ParseTokenTTL("bogus")
	want := 168 * time.Hour
	if got != want {
		t.Fatalf("ParseTokenTTL(\"bogus\") = %v, want %v (default)", got, want)
	}
}

func TestBuildStores_InMemory(t *testing.T) {
	c := config.Default()
	stores, err := config.BuildStores(t.Context(), c, nil)
	if err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
	if stores.Cache == nil {
		t.Fatal("Cache store is nil")
	}
	if stores.Vault == nil {
		t.Fatal("Vault store is nil")
	}
	if stores.WatchHistory == nil {
		t.Fatal("WatchHistory store is nil")
	}
	if stores.Jobs == nil {
		t.Fatal("Jobs store is nil")
	}
	if stores.Sessions == nil {
		t.Fatal("Sessions store is nil")
	}
	if stores.Users == nil {
		t.Fatal("Users store is nil")
	}
	if stores.SourceCache == nil {
		t.Fatal("SourceCache store is nil")
	}
}

func TestBuildStores_VaultDefaultPath(t *testing.T) {
	dir := t.TempDir()
	c := config.Default()
	c.Stores.Vault = config.StoreConfig{Backend: "local-file", Path: filepath.Join(dir, "vault.db")}
	c.Stores.SourceCache = config.StoreConfig{Backend: "local-file", Path: filepath.Join(dir, "source-cache.db")}
	c.Stores.VaultKey = "generated"
	_, err := config.BuildStores(t.Context(), c, nil)
	if err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
}

func TestBuildStores_UserRecordsFollowTheInstanceKey(t *testing.T) {
	dir := t.TempDir()
	usersPath := filepath.Join(dir, "users.db")

	newCfg := func(key string) *config.Config {
		c := config.Default()
		c.Stores.VaultKey = key
		c.Stores.Users = config.StoreConfig{Backend: "local-file", Path: usersPath}
		return c
	}

	first, err := config.BuildStores(t.Context(), newCfg(strings.Repeat("ab", 32)), nil)
	if err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
	users := auth.NewStoreUserStore(first.Users)
	if err := users.PutUser("alice", &auth.UserData{Salt: []byte("s"), PasswordHash: []byte("h"), WrappedDEK: []byte("d"), WrappedRecovery: []byte("r")}); err != nil {
		t.Fatalf("PutUser: %v", err)
	}
	if err := config.CloseStores(first); err != nil {
		t.Fatalf("CloseStores: %v", err)
	}

	// The on-disk record must never be the plaintext JSON of a login.
	var leaked bool
	if blobs, err := filepath.Glob(usersPath + "*"); err == nil {
		for _, f := range blobs {
			raw, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			if bytes.Contains(raw, []byte("PasswordHash")) {
				leaked = true
			}
		}
	}
	if leaked {
		t.Fatal("a login record reached disk as plaintext JSON")
	}

	// A restart with the same key must read the record back.
	second, err := config.BuildStores(t.Context(), newCfg(strings.Repeat("ab", 32)), nil)
	if err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
	if _, err := auth.NewStoreUserStore(second.Users).GetUser("alice"); err != nil {
		t.Fatalf("a sealed record must open under the same instance key: %v", err)
	}
	if err := config.CloseStores(second); err != nil {
		t.Fatalf("CloseStores: %v", err)
	}

	// The same file under a different key must fail closed — nothing opens.
	third, err := config.BuildStores(t.Context(), newCfg(strings.Repeat("cd", 32)), nil)
	if err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
	if _, err := auth.NewStoreUserStore(third.Users).GetUser("alice"); err == nil {
		t.Fatal("a sealed record opened under a different instance key")
	}
}

func TestBuildStores_DurabilityAuditReportsInMemoryAndGeneratedKey(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	if _, err := config.BuildStores(t.Context(), config.Default(), logger); err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"no instance key configured",
		"logins (users)",
		"account sessions (vault)",
		"watch history",
		"jobs are in-memory",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("durability audit missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestBuildStores_DurableConfigProducesNoDurabilityWarnings(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	dir := t.TempDir()
	c := config.Default()
	c.Stores.VaultKey = strings.Repeat("cd", 32)
	c.Stores.Vault = config.StoreConfig{Backend: "local-file", Path: filepath.Join(dir, "vault.db")}
	c.Stores.Users = config.StoreConfig{Backend: "local-file", Path: filepath.Join(dir, "users.db")}
	c.Stores.WatchHistory = config.StoreConfig{Backend: "local-file", Path: filepath.Join(dir, "watch-history.db")}
	c.Stores.Jobs = config.StoreConfig{Backend: "local-file", Path: filepath.Join(dir, "jobs.db")}
	if _, err := config.BuildStores(t.Context(), c, logger); err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("a durable configuration must not warn; got:\n%s", buf.String())
	}
}

func TestBuildStores_UnknownBackend(t *testing.T) {
	c := config.Default()
	c.Stores.Caches = config.StoreConfig{Backend: "bogus"}
	_, err := config.BuildStores(t.Context(), c, nil)
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestBuildAuth(t *testing.T) {
	userStore := store.NewInMemory()
	sessionStore := store.NewInMemory()
	users, tokens, deks, err := config.BuildAuth(userStore, sessionStore, "memory", nil)
	if err != nil {
		t.Fatalf("BuildAuth: %v", err)
	}
	if users == nil {
		t.Fatal("UserStore is nil")
	}
	if tokens == nil {
		t.Fatal("TokenStore is nil")
	}
	if deks == nil {
		t.Fatal("DEKCache is nil")
	}
}

func TestBuildAuth_DEKCacheModes(t *testing.T) {
	userStore := store.NewInMemory()
	sessionStore := store.NewInMemory()

	// Empty mode means memory (default).
	_, _, _, err := config.BuildAuth(userStore, sessionStore, "", nil)
	if err != nil {
		t.Fatalf("BuildAuth(empty): %v", err)
	}
	if _, _, _, err := config.BuildAuth(userStore, sessionStore, "encrypted-store", nil); err == nil {
		t.Fatal("expected error for encrypted-store without cipher")
	}
	if _, _, _, err := config.BuildAuth(userStore, sessionStore, "bogus", nil); err == nil {
		t.Fatal("expected error for unknown dek-cache mode")
	}
}

func TestBuildSession(t *testing.T) {
	userStore := store.NewInMemory()
	sessionStore := store.NewInMemory()
	_, tokStore, dekCache, err := config.BuildAuth(userStore, sessionStore, "memory", nil)
	if err != nil {
		t.Fatalf("BuildAuth: %v", err)
	}
	session := config.BuildSession(tokStore, dekCache, time.Hour)
	if session == nil {
		t.Fatal("BuildSession returned nil")
	}
}

func TestBuildAuthenticator_Password(t *testing.T) {
	userStore := store.NewInMemory()
	users, _, _, err := config.BuildAuth(userStore, store.NewInMemory(), "memory", nil)
	if err != nil {
		t.Fatalf("BuildAuth: %v", err)
	}
	composite, err := config.BuildAuthenticator([]string{"password"}, users)
	if err != nil {
		t.Fatalf("BuildAuthenticator: %v", err)
	}
	a, ok := composite.Get("password")
	if !ok {
		t.Fatal("password method not found")
	}
	if a == nil {
		t.Fatal("password authenticator is nil")
	}
}

func TestBuildAuthenticator_Unknown(t *testing.T) {
	userStore := store.NewInMemory()
	users, _, _, err := config.BuildAuth(userStore, store.NewInMemory(), "memory", nil)
	if err != nil {
		t.Fatalf("BuildAuth: %v", err)
	}
	_, err = config.BuildAuthenticator([]string{"oauth"}, users)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestLoad_SlotLists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := []byte(`
slots:
  builtin:
    enabled: false
  providers:
    - adapter: jellyfin
      id: primary
      enabled: true
      server: http://jellyfin.local:8096
      sync-cadence: 15m
      accounts:
        - id: primary
          username: bob
          password-env: JELLYFIN_PASSWORD
`)
	if err := os.WriteFile(path, yaml, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Slots.Builtin.Enabled {
		t.Fatal("builtin should be disabled by the file")
	}
	if len(c.Slots.Providers) != 1 {
		t.Fatalf("providers = %d entries, want 1", len(c.Slots.Providers))
	}
	p := c.Slots.Providers[0]
	if p.Adapter != "jellyfin" || p.ID != "primary" || !p.Enabled || p.SyncCadence != "15m" {
		t.Fatalf("provider entry mismatch: %+v", p)
	}
	if len(p.Accounts) != 1 || p.Accounts[0].PasswordEnv != "JELLYFIN_PASSWORD" {
		t.Fatalf("accounts mismatch: %+v", p.Accounts)
	}
}

func TestValidateSlots_Rejections(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "missing id",
			yaml:    "slots:\n  providers:\n    - adapter: jellyfin\n      enabled: true\n",
			wantErr: "id is required",
		},
		{
			name:    "missing adapter",
			yaml:    "slots:\n  providers:\n    - id: primary\n      enabled: true\n      server: \"http://jf.example\"\n",
			wantErr: "adapter is required",
		},
		{
			name:    "missing server",
			yaml:    "slots:\n  providers:\n    - id: primary\n      adapter: jellyfin\n      enabled: true\n",
			wantErr: "must declare the server",
		},
		{
			name:    "account missing id",
			yaml:    "slots:\n  providers:\n    - id: primary\n      adapter: jellyfin\n      enabled: true\n      server: \"http://jf.example\"\n      accounts:\n        - username: bob\n",
			wantErr: "account entry missing id",
		},
		{
			name: "duplicate id across kinds",
			yaml: `slots:
  providers:
    - adapter: jellyfin
      id: primary
      enabled: true
      server: "http://jf.example"
  sinks:
    - adapter: jellyfin
      id: primary
      enabled: true
`,
			wantErr: `duplicate slot id "primary"`,
		},
		{
			name:    "unsupported transport",
			yaml:    "slots:\n  providers:\n    - adapter: jellyfin\n      id: primary\n      transport: subprocess\n      enabled: true\n",
			wantErr: `unsupported transport "subprocess"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}
			_, err := config.Load(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

// The instance cap-change default parses loudly: the named modes resolve, an
// empty value is the shipped default (new sessions only — running playback
// is never interrupted out of the box), and anything else is a startup
// failure rather than a silent fallback.
func TestParseCapChangeDefault(t *testing.T) {
	for raw, want := range map[string]accounts.CapChangePolicy{
		"":                  accounts.CapChangePolicyNewSessionsOnly,
		"new-sessions-only": accounts.CapChangePolicyNewSessionsOnly,
		"enforce-now":       accounts.CapChangePolicyEnforceNow,
	} {
		got, err := config.ParseCapChangeDefault(raw)
		if err != nil {
			t.Fatalf("config.ParseCapChangeDefault(%q): %v", raw, err)
		}
		if got != want {
			t.Errorf("config.ParseCapChangeDefault(%q) = %q, want %q", raw, got, want)
		}
	}
	if _, err := config.ParseCapChangeDefault("explode"); err == nil {
		t.Fatal("unknown value should fail")
	}
}

// The delivery timing knobs resolve like every other operator knob: absent
// keys inherit the shipped defaults, explicit values win, and a malformed or
// non-positive duration is a startup error naming the key — never a silent
// fallback (TECHNICAL-DECISIONS.md §1.14).
func TestParseDeliveryTiming(t *testing.T) {
	def, err := config.ParseDeliveryTiming(config.DeliveryConfig{})
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if def.SessionTTL != 24*time.Hour ||
		def.HeartbeatInterval != 30*time.Second ||
		def.HeartbeatGrace != 90*time.Second {
		t.Fatalf("defaults = %+v, want 24h/30s/90s", def)
	}

	over, err := config.ParseDeliveryTiming(config.DeliveryConfig{
		SessionTTL: "48h",
		Heartbeat: config.HeartbeatConfig{
			Interval: "45s",
			Grace:    "2m",
		},
	})
	if err != nil {
		t.Fatalf("overrides: %v", err)
	}
	if over.SessionTTL != 48*time.Hour ||
		over.HeartbeatInterval != 45*time.Second ||
		over.HeartbeatGrace != 2*time.Minute {
		t.Fatalf("overrides = %+v, want 48h/45s/2m", over)
	}

	// A key left absent inside a present block still inherits its default.
	partial, err := config.ParseDeliveryTiming(config.DeliveryConfig{
		Heartbeat: config.HeartbeatConfig{Interval: "45s"},
	})
	if err != nil {
		t.Fatalf("partial: %v", err)
	}
	if partial.HeartbeatInterval != 45*time.Second || partial.HeartbeatGrace != 90*time.Second {
		t.Fatalf("partial = %+v, want interval 45s with grace default 90s", partial)
	}
}

func TestParseDeliveryTimingRejectsBrokenValues(t *testing.T) {
	for name, cfg := range map[string]config.DeliveryConfig{
		"session-ttl malformed":  {SessionTTL: "banana"},
		"session-ttl zero":       {SessionTTL: "0s"},
		"interval malformed":     {Heartbeat: config.HeartbeatConfig{Interval: "soon"}},
		"interval negative":      {Heartbeat: config.HeartbeatConfig{Interval: "-5s"}},
		"grace malformed":        {Heartbeat: config.HeartbeatConfig{Grace: "90"}},
		"grace non-positive":     {Heartbeat: config.HeartbeatConfig{Grace: "0s"}},
	} {
		if _, err := config.ParseDeliveryTiming(cfg); err == nil {
			t.Errorf("%s: broken value should fail", name)
		}
	}
}

// The matching article list fails loudly on entries that can never fire:
// normalization compares lowercase single tokens, so anything else is a typo
// the operator must hear about at startup.
func TestValidateArticles(t *testing.T) {
	for _, ok := range [][]string{nil, {}, {"the", "a", "an"}, {"le", "la", "les"}, {"der"}} {
		if err := config.ValidateArticles(ok); err != nil {
			t.Errorf("ValidateArticles(%v): %v", ok, err)
		}
	}
	for _, bad := range [][]string{{"The"}, {"le "}, {""}, {"a b"}} {
		if err := config.ValidateArticles(bad); err == nil {
			t.Errorf("ValidateArticles(%v) should fail", bad)
		}
	}
}

// Broken delivery/library values refuse the whole config load, not just the
// parse helper — boot must never run on a silently-fallen-back timing or a
// dead article entry.
func TestLoadRejectsBrokenDeliveryAndLibrary(t *testing.T) {
	for name, yaml := range map[string]string{
		"bad session-ttl":   "delivery:\n  session-ttl: banana\n",
		"bad grace":         "delivery:\n  heartbeat:\n    grace: \"0s\"\n",
		"uppercase article": "library:\n  articles: [The, le]\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := config.Load(path); err == nil {
				t.Fatalf("%s: broken value should fail the load", name)
			}
		})
	}
}

// Valid delivery and library blocks parse into the config struct.
func TestLoadDeliveryAndLibrary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `delivery:
  on-cap-change: enforce-now
  session-ttl: 12h
  heartbeat:
    interval: 20s
    grace: 1m
library:
  articles: [le, la, les]
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Delivery.SessionTTL != "12h" ||
		c.Delivery.Heartbeat.Interval != "20s" ||
		c.Delivery.Heartbeat.Grace != "1m" {
		t.Fatalf("delivery = %+v", c.Delivery)
	}
	if len(c.Library.Articles) != 3 || c.Library.Articles[2] != "les" {
		t.Fatalf("articles = %v", c.Library.Articles)
	}
}
