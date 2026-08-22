package store

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const vaultSchema = `
CREATE TABLE IF NOT EXISTS vault (
    key       TEXT PRIMARY KEY,
    nonce     BLOB NOT NULL,
    ciphertext BLOB NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// Vault is a Store that persists encrypted key-value pairs in a single SQLite
// database file (PLAN.md §2.4: "durable, must not lose, owner's key at rest").
//
// Every value is encrypted with the provided AEAD cipher before writing and
// decrypted on read. The store never holds the encryption key — the caller
// constructs the cipher from the vault's master key and passes it at creation
// time.
//
// The on-disk format per value is: [12-byte nonce][ciphertext + 16-byte GCM
// tag]. The nonce is prepended so it can be recovered on read; it is not secret.
type Vault struct {
	db   *sql.DB
	aead cipher.AEAD
}

// NewVault opens or creates a SQLite database at path and returns a Vault that
// encrypts all values with aead. The caller must ensure aead was constructed
// from a key the server never persists in plaintext (PLAN.md §7.6).
func NewVault(ctx context.Context, path string, aead cipher.AEAD) (*Vault, error) {
	if aead == nil {
		return nil, fmt.Errorf("vault: aead cipher is required")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("vault: open %s: %w", path, err)
	}
	// WAL mode gives better concurrent read performance and is the
	// recommended mode for most SQLite workloads.
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("vault: enable WAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, vaultSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("vault: create schema: %w", err)
	}
	return &Vault{db: db, aead: aead}, nil
}

func (v *Vault) Get(ctx context.Context, key string) ([]byte, error) {
	var nonce, ciphertext []byte
	err := v.db.QueryRowContext(ctx,
		"SELECT nonce, ciphertext FROM vault WHERE key = ?", key,
	).Scan(&nonce, &ciphertext)
	if err == sql.ErrNoRows {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("vault: get %q: %w", key, err)
	}
	plaintext, err := v.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("vault: decrypt %q: %w", key, err)
	}
	return plaintext, nil
}

func (v *Vault) Put(ctx context.Context, key string, value []byte) error {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("vault: generate nonce: %w", err)
	}
	ciphertext := v.aead.Seal(nil, nonce, value, nil)
	_, err := v.db.ExecContext(ctx,
		`INSERT INTO vault (key, nonce, ciphertext, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		   nonce = excluded.nonce,
		   ciphertext = excluded.ciphertext,
		   updated_at = excluded.updated_at`,
		key, nonce, ciphertext, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("vault: put %q: %w", key, err)
	}
	return nil
}

func (v *Vault) Delete(ctx context.Context, key string) error {
	_, err := v.db.ExecContext(ctx, "DELETE FROM vault WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("vault: delete %q: %w", key, err)
	}
	return nil
}

func (v *Vault) List(ctx context.Context, prefix string) ([]string, error) {
	rows, err := v.db.QueryContext(ctx,
		"SELECT key FROM vault WHERE key LIKE ?", prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("vault: list prefix=%q: %w", prefix, err)
	}
	defer func() { _ = rows.Close() }()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("vault: list scan: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vault: list iter: %w", err)
	}
	return keys, nil
}

func (v *Vault) Close() error {
	return v.db.Close()
}

// CiphertextOnDisk reads the raw (encrypted) value for key directly from the
// database, without decrypting. This is used in tests to prove that plaintext
// never appears on disk (TESTING.md §6).
func (v *Vault) CiphertextOnDisk(ctx context.Context, key string) ([]byte, error) {
	var ciphertext []byte
	err := v.db.QueryRowContext(ctx,
		"SELECT ciphertext FROM vault WHERE key = ?", key,
	).Scan(&ciphertext)
	if err == sql.ErrNoRows {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("vault: raw read %q: %w", key, err)
	}
	return ciphertext, nil
}

// RawSQL is a helper for tests that need to inspect the database directly.
// It runs the query and returns the first row's first column as a string.
func (v *Vault) RawSQL(ctx context.Context, query string, args ...any) (string, error) {
	var result string
	err := v.db.QueryRowContext(ctx, query, args...).Scan(&result)
	if err != nil {
		return "", fmt.Errorf("vault: raw query: %w", err)
	}
	return result, nil
}

// hasPlaintext is a test helper that checks if a byte slice contains any
// occurrence of the given substring. Used to prove ciphertext on disk does
// not contain plaintext.
func containsBytes(haystack []byte, needle string) bool {
	return strings.Contains(string(haystack), needle)
}
