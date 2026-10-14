package settings_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/settings"
	"github.com/nem-git/abcmovies/core/internal/store"
)

func testDefaults() settings.Snapshot {
	return settings.Snapshot{
		AuthMethods:        []string{"password"},
		HeartbeatInterval:  config.DefaultHeartbeatInterval,
		HeartbeatGrace:     config.DefaultHeartbeatGrace,
		SessionTTL:         config.DefaultSessionTTL,
		OwnerUserIDVisible: false,
	}
}

func mustNew(t *testing.T, backend store.Store, defaults settings.Snapshot) *settings.Provider {
	t.Helper()
	p, err := settings.New(backend, defaults)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

// A provider over an empty store reports the shipped defaults: an instance
// with no stored record and no config override runs on the defaults, not on
// zero values (A1).
func TestNew_UsesDefaultsWhenStoreEmpty(t *testing.T) {
	p := mustNew(t, store.NewInMemory(), testDefaults())

	got, err := p.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.AuthMethods) != 1 || got.AuthMethods[0] != "password" {
		t.Errorf("auth methods = %v, want [password]", got.AuthMethods)
	}
	if got.HeartbeatInterval != config.DefaultHeartbeatInterval || got.HeartbeatGrace != config.DefaultHeartbeatGrace || got.SessionTTL != config.DefaultSessionTTL {
		t.Errorf("timing = %v/%v/%v, want defaults", got.HeartbeatInterval, got.HeartbeatGrace, got.SessionTTL)
	}
	if got.OwnerUserIDVisible {
		t.Errorf("owner_user_id_visible = true, want default false")
	}
}

// Boot persists the resolved settings so a second boot reads the same record
// (A1): the record, not the config, is the durable home.
func TestApply_PersistsForNextBoot(t *testing.T) {
	backend := store.NewInMemory()
	p := mustNew(t, backend, testDefaults())

	ttl := 48 * time.Hour
	if _, err := p.Apply(context.Background(), settings.Patch{SessionTTL: &ttl}, settings.LiveApply{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// A fresh provider over the same backend sees the persisted value, even
	// though the shipped default differs.
	next := mustNew(t, backend, testDefaults())
	got, err := next.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SessionTTL != 48*time.Hour {
		t.Errorf("reloaded session_ttl = %v, want 48h", got.SessionTTL)
	}
}

// E2: the pipeline hands each resolved value to its live consumer before it
// persists anything, and the consumer sees the new value.
func TestApply_HandsValuesToLiveConsumerBeforePersist(t *testing.T) {
	backend := store.NewInMemory()
	p := mustNew(t, backend, testDefaults())

	var seenTTL time.Duration
	var seenMethods []string
	live := settings.LiveApply{
		SessionTTL:  func(d time.Duration) error { seenTTL = d; return nil },
		AuthMethods: func(m []string) error { seenMethods = append([]string(nil), m...); return nil },
	}

	ttl := time.Hour
	methods := []string{"password", "password"}
	if _, err := p.Apply(context.Background(), settings.Patch{SessionTTL: &ttl, AuthMethods: &methods}, live); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if seenTTL != time.Hour {
		t.Errorf("live consumer saw session_ttl %v, want 1h", seenTTL)
	}
	if len(seenMethods) != 2 {
		t.Errorf("live consumer saw methods %v, want the patched set", seenMethods)
	}
}

// E2: an invalid resolved value is rejected, and the store is left untouched —
// a refused write never half-persists.
func TestApply_RejectsInvalidValueWithoutPersisting(t *testing.T) {
	backend := store.NewInMemory()
	p := mustNew(t, backend, testDefaults())

	zero := time.Duration(0)
	if _, err := p.Apply(context.Background(), settings.Patch{SessionTTL: &zero}, settings.LiveApply{}); err == nil {
		t.Fatal("Apply with non-positive session_ttl: want error, got nil")
	}

	// The rejected value did not reach the store: the record still holds the
	// default.
	next := mustNew(t, backend, testDefaults())
	got, err := next.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SessionTTL != config.DefaultSessionTTL {
		t.Errorf("session_ttl after rejected write = %v, want untouched default", got.SessionTTL)
	}
}

// E2: a value the live system cannot apply is rejected, not persisted for the
// next restart to trip over.
func TestApply_RejectsWhenLiveApplyFails(t *testing.T) {
	backend := store.NewInMemory()
	p := mustNew(t, backend, testDefaults())

	methods := []string{"password"}
	live := settings.LiveApply{AuthMethods: func([]string) error { return errors.New("unknown method") }}
	if _, err := p.Apply(context.Background(), settings.Patch{AuthMethods: &methods}, live); err == nil {
		t.Fatal("Apply when live apply fails: want error, got nil")
	}

	next := mustNew(t, backend, testDefaults())
	got, err := next.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.AuthMethods) != 1 {
		t.Errorf("auth methods after rejected apply = %v, want untouched", got.AuthMethods)
	}
}

// A patch that names only some settings leaves the rest at their stored
// values: an absent key never resets a setting to the shipped default.
func TestApply_AbsentFieldKeepsStoredValue(t *testing.T) {
	backend := store.NewInMemory()
	p := mustNew(t, backend, testDefaults())

	ttl := 48 * time.Hour
	if _, err := p.Apply(context.Background(), settings.Patch{SessionTTL: &ttl}, settings.LiveApply{}); err != nil {
		t.Fatalf("Apply ttl: %v", err)
	}

	visible := true
	if _, err := p.Apply(context.Background(), settings.Patch{OwnerUserIDVisible: &visible}, settings.LiveApply{}); err != nil {
		t.Fatalf("Apply visible: %v", err)
	}

	got, err := p.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SessionTTL != 48*time.Hour {
		t.Errorf("session_ttl = %v, want the stored 48h (absent field must not reset)", got.SessionTTL)
	}
	if !got.OwnerUserIDVisible {
		t.Errorf("owner_user_id_visible = false, want true")
	}
}

// A corrupt persisted record fails boot loudly rather than silently falling
// back to defaults and discarding the operator's settings.
func TestNew_RejectsCorruptRecord(t *testing.T) {
	backend := store.NewInMemory()
	if err := backend.Put(context.Background(), settings.StorageKey, []byte("{not json")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := settings.New(backend, testDefaults()); err == nil {
		t.Fatal("New over a corrupt record: want error, got nil")
	}
}

// A stored record that is valid JSON but violates a setting rule is also a
// boot failure, not a silent default.
func TestNew_RejectsInvalidStoredRecord(t *testing.T) {
	backend := store.NewInMemory()
	if err := backend.Put(context.Background(), settings.StorageKey, []byte(`{"auth_methods":[],"heartbeat_interval_ns":1,"heartbeat_grace_ns":1,"session_ttl_ns":1}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := settings.New(backend, testDefaults()); err == nil {
		t.Fatal("New over an invalid record: want error, got nil")
	}
}

// Get hands out an independent copy: a caller cannot mutate the provider's
// live state through the returned value.
func TestGet_ReturnsIndependentCopy(t *testing.T) {
	p := mustNew(t, store.NewInMemory(), testDefaults())

	got, err := p.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got.AuthMethods[0] = "mutated"

	again, err := p.Get(context.Background())
	if err != nil {
		t.Fatalf("Get again: %v", err)
	}
	if again.AuthMethods[0] != "password" {
		t.Errorf("provider state mutated through Get: %v", again.AuthMethods)
	}
}
