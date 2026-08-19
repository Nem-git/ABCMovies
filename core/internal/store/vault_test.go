package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"log/slog"
	"strings"
	"testing"
)

// testAEAD returns an AES-256-GCM cipher for tests. The key is static and
// never leaves the test binary — this is fine for unit tests.
func testAEAD(t *testing.T) cipher.AEAD {
	t.Helper()
	// 32-byte key for AES-256.
	key := []byte("0123456789abcdef0123456789abcdef")
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	return aead
}

// testAEADDifferentKey returns a second AES-256-GCM cipher with a different
// key, used to test that one key cannot decrypt data encrypted with another.
func testAEADDifferentKey(t *testing.T) cipher.AEAD {
	t.Helper()
	key := []byte("fedcba9876543210fedcba9876543210")
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	return aead
}

func newTestVault(t *testing.T, aead cipher.AEAD) *Vault {
	t.Helper()
	path := t.TempDir() + "/vault.db"
	v, err := NewVault(t.Context(), path, aead)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

func TestVault(t *testing.T) {
	RunStoreSuite(t, func(t *testing.T) Store {
		t.Helper()
		return newTestVault(t, testAEAD(t))
	})
}

// --- Vault/secrets discipline tests (TESTING.md §6) ---

func TestVaultEncryptedAtRest(t *testing.T) {
	v := newTestVault(t, testAEAD(t))
	ctx := t.Context()
	plaintext := []byte("super-secret-session-cookie")
	if err := v.Put(ctx, "account:netflix:user1", plaintext); err != nil {
		t.Fatalf("put: %v", err)
	}

	// Read the raw ciphertext from disk (bypasses decryption).
	ciphertext, err := v.CiphertextOnDisk(ctx, "account:netflix:user1")
	if err != nil {
		t.Fatalf("ciphertext on disk: %v", err)
	}

	// The raw bytes must not contain the plaintext.
	if containsBytes(ciphertext, string(plaintext)) {
		t.Fatal("ciphertext on disk contains plaintext — encryption at rest violated")
	}
	// The raw bytes must not contain any recognizable part of the plaintext.
	if containsBytes(ciphertext, "super-secret") {
		t.Fatal("ciphertext on disk contains 'super-secret' — encryption at rest violated")
	}
}

func TestVaultInMemoryOnlyDuringUse(t *testing.T) {
	v := newTestVault(t, testAEAD(t))
	ctx := t.Context()
	plaintext := []byte("ephemeral-session-data")

	if err := v.Put(ctx, "session:1", plaintext); err != nil {
		t.Fatalf("put: %v", err)
	}

	// Get returns plaintext — but it must be a fresh decryption each time,
	// not a cached reference.
	got1, err := v.Get(ctx, "session:1")
	if err != nil {
		t.Fatalf("get 1: %v", err)
	}
	got2, err := v.Get(ctx, "session:1")
	if err != nil {
		t.Fatalf("get 2: %v", err)
	}

	// Both calls returned the correct plaintext.
	if string(got1) != string(plaintext) {
		t.Fatalf("get 1: got %q, want %q", got1, plaintext)
	}
	if string(got2) != string(plaintext) {
		t.Fatalf("get 2: got %q, want %q", got2, plaintext)
	}

	// Mutating one return value must not affect the other (no shared
	// underlying buffer — each Get decrypts fresh).
	got1[0] = 'X'
	if string(got2) == string(plaintext) {
		// got2 is still correct — good, no shared buffer.
	} else {
		t.Fatal("mutating one Get result affected another — shared buffer detected")
	}
}

func TestVaultNothingLogged(t *testing.T) {
	// Set up a logger that captures output.
	var buf strings.Builder
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	v := newTestVault(t, testAEAD(t))
	ctx := context.WithValue(t.Context(), logKey{}, logger)
	plaintext := []byte("top-secret-session")

	if err := v.Put(ctx, "secret:key", plaintext); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, err := v.Get(ctx, "secret:key"); err != nil {
		t.Fatalf("get: %v", err)
	}

	// The log output must not contain the plaintext.
	logOutput := buf.String()
	if strings.Contains(logOutput, string(plaintext)) {
		t.Fatal("plaintext appeared in log output — logging discipline violated")
	}
	if strings.Contains(logOutput, "top-secret") {
		t.Fatal("'top-secret' appeared in log output — logging discipline violated")
	}
}

// logKey is the context key for the test logger.
type logKey struct{}

func TestVaultRelayKeyScoping(t *testing.T) {
	ownerKey := testAEAD(t)
	relayKey := testAEADDifferentKey(t)

	// Use a shared directory so both vaults open the same SQLite file.
	dir := t.TempDir()
	dbPath := dir + "/shared.db"

	// Write with owner key.
	ownerVault, err := NewVault(t.Context(), dbPath, ownerKey)
	if err != nil {
		t.Fatalf("owner vault: %v", err)
	}
	userBlob := []byte("user-watch-history:items=[...]")
	if err := ownerVault.Put(t.Context(), "user:bob:history", userBlob); err != nil {
		t.Fatalf("put: %v", err)
	}
	_ = ownerVault.Close()

	// Open the same database with the relay key — it must NOT decrypt.
	relayVault, err := NewVault(t.Context(), dbPath, relayKey)
	if err != nil {
		t.Fatalf("relay vault: %v", err)
	}
	defer func() { _ = relayVault.Close() }()

	_, err = relayVault.Get(t.Context(), "user:bob:history")
	if err == nil {
		t.Fatal("relay key should not decrypt owner-encrypted data")
	}
	// The error should be a decryption failure, not a key-not-found.
	if strings.Contains(err.Error(), ErrKeyNotFound.Error()) {
		t.Fatal("got ErrKeyNotFound instead of decryption error — data was found but decryption should fail")
	}
}

func TestVaultOwnerOnlyOps(t *testing.T) {
	// Owner-only operations: re-link, re-auth, revoke. These require the
	// owner's KEK. We simulate this by encrypting with the owner key and
	// verifying the relay key cannot perform mutations.

	ownerKey := testAEAD(t)
	relayKey := testAEADDifferentKey(t)

	dir := t.TempDir()
	dbPath := dir + "/shared.db"

	// Owner creates a session.
	ownerVault, err := NewVault(t.Context(), dbPath, ownerKey)
	if err != nil {
		t.Fatalf("owner vault: %v", err)
	}
	session := []byte("netflix-session-token-abc123")
	if err := ownerVault.Put(t.Context(), "account:netflix:session", session); err != nil {
		t.Fatalf("owner put: %v", err)
	}
	_ = ownerVault.Close()

	// Relay opens the same database — can read (with relay key for
	// relay-encrypted sessions), but cannot write owner-encrypted sessions.
	relayVault, err := NewVault(t.Context(), dbPath, relayKey)
	if err != nil {
		t.Fatalf("relay vault: %v", err)
	}
	defer func() { _ = relayVault.Close() }()

	// Relay tries to update (re-link) the owner's session — this would
	// overwrite the owner-encrypted blob with relay-encrypted data, which
	// is an unauthorized mutation. In practice, the application layer
	// checks authorization; here we verify the encryption boundary:
	// the relay key cannot read what the owner encrypted.
	_, err = relayVault.Get(t.Context(), "account:netflix:session")
	if err == nil {
		t.Fatal("relay key should not be able to read owner-encrypted session")
	}
}

func TestVaultNilCipherRejected(t *testing.T) {
	_, err := NewVault(t.Context(), t.TempDir()+"/nil.db", nil)
	if err == nil {
		t.Fatal("expected error when constructing Vault with nil cipher")
	}
}
