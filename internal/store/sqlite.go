// Package store implements SQLite-backed persistence.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Admin represents a local admin account.
type Admin struct {
	Username string
	Hash     string
	Created  time.Time
}

// Session represents a logged-in session.
type Session struct {
	ID       string
	Username string
	Created  time.Time
	Expires  time.Time
}

const schema = `
CREATE TABLE IF NOT EXISTS admins (
    username  TEXT PRIMARY KEY,
    hash      TEXT NOT NULL,
    created   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS sessions (
    id        TEXT PRIMARY KEY,
    username  TEXT NOT NULL REFERENCES admins(username) ON DELETE CASCADE,
    created   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires   TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_expires ON sessions(expires);
`

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (and migrates) the store at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// AdminCount returns how many admin accounts exist.
func (s *Store) AdminCount(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admins`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return n, nil
}

// CreateAdmin stores a new admin account. It errors if the admin already exists.
func (s *Store) CreateAdmin(ctx context.Context, username, hash string) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO admins (username, hash) VALUES (?, ?)`, username, hash); err != nil {
		return fmt.Errorf("insert admin: %w", err)
	}
	return nil
}

// GetAdmin returns the admin with the given username, or sql.ErrNoRows.
func (s *Store) GetAdmin(ctx context.Context, username string) (*Admin, error) {
	a := &Admin{}
	if err := s.db.QueryRowContext(ctx,
		`SELECT username, hash, created FROM admins WHERE username = ?`, username).Scan(&a.Username, &a.Hash, &a.Created); err != nil {
		return nil, err
	}
	return a, nil
}

// CreateSession creates a session for username with the given lifetime.
func (s *Store) CreateSession(ctx context.Context, username string, ttl time.Duration) (*Session, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	sess := &Session{
		ID:       hex.EncodeToString(token),
		Username: username,
		Created:  time.Now(),
		Expires:  time.Now().Add(ttl),
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, username, expires) VALUES (?, ?, ?)`,
		sess.ID, sess.Username, sess.Expires); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return sess, nil
}

// ValidateSession returns the session if it exists and has not expired, or sql.ErrNoRows.
func (s *Store) ValidateSession(ctx context.Context, id string) (*Session, error) {
	sess := &Session{}
	if err := s.db.QueryRowContext(ctx,
		`SELECT id, username, created, expires FROM sessions WHERE id = ? AND expires > CURRENT_TIMESTAMP`, id).Scan(&sess.ID, &sess.Username, &sess.Created, &sess.Expires); err != nil {
		return nil, err
	}
	return sess, nil
}

// DeleteSession removes the session with the given id, if any.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}