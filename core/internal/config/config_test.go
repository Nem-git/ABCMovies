package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
}

func TestBuildStores_VaultDefaultPath(t *testing.T) {
	dir := t.TempDir()
	c := config.Default()
	c.Stores.Vault = config.StoreConfig{Backend: "local-file", Path: filepath.Join(dir, "vault.db")}
	c.Stores.VaultKey = "generated"
	_, err := config.BuildStores(t.Context(), c, nil)
	if err != nil {
		t.Fatalf("BuildStores: %v", err)
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
