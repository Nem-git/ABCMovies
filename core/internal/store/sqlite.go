package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS kv (
    key        TEXT PRIMARY KEY,
    value      BLOB NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// SQLite is a Store backed by a single SQLite database file with no
// application-level encryption. Suitable for rebuildable caches, checkpointed
// job state, and other durable-but-not-secret store classes (PLAN.md §2.4).
//
// For stores that require per-value encryption (vault, user blobs), use Vault
// instead. The on-disk format is plain key-value; a leaked database file yields
// cleartext.
type SQLite struct {
	db *sql.DB
}

// NewSQLite opens or creates a SQLite database at path and returns a ready
// Store. The caller should call Close when done.
func NewSQLite(_ context.Context, path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", path, err)
	}
	if _, err := db.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: enable WAL: %w", err)
	}
	if _, err := db.ExecContext(context.Background(), sqliteSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: create schema: %w", err)
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) Get(_ context.Context, key string) ([]byte, error) {
	var value []byte
	err := s.db.QueryRowContext(context.Background(),
		"SELECT value FROM kv WHERE key = ?", key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get %q: %w", key, err)
	}
	out := make([]byte, len(value))
	copy(out, value)
	return out, nil
}

func (s *SQLite) Put(_ context.Context, key string, value []byte) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO kv (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		   value = excluded.value,
		   updated_at = excluded.updated_at`,
		key, value, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("sqlite: put %q: %w", key, err)
	}
	return nil
}

func (s *SQLite) Delete(_ context.Context, key string) error {
	_, err := s.db.ExecContext(context.Background(), "DELETE FROM kv WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("sqlite: delete %q: %w", key, err)
	}
	return nil
}

func (s *SQLite) List(_ context.Context, prefix string) ([]string, error) {
	rows, err := s.db.QueryContext(context.Background(),
		"SELECT key FROM kv WHERE key LIKE ?", prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list prefix=%q: %w", prefix, err)
	}
	defer func() { _ = rows.Close() }()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("sqlite: list scan: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: list iter: %w", err)
	}
	return keys, nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

// Keys returns all keys in the database, sorted. Used in tests.
func (s *SQLite) Keys(_ context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(context.Background(), "SELECT key FROM kv ORDER BY key")
	if err != nil {
		return nil, fmt.Errorf("sqlite: keys: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("sqlite: keys scan: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: keys iter: %w", err)
	}
	return keys, nil
}
