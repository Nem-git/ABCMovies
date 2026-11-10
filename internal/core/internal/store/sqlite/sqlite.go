// Package sqlite implements the SQLite-backed persistence engine for the
// core store. Domain-specific queries are split by domain file
// (users.go, sessions.go, sources.go, library.go); this file owns the
// connection, schema migration, and shared helpers.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"modernc.org/sqlite"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// ErrAlreadyInitialized is returned when the instance already has an initial
// user and cannot be initialized again.
var ErrAlreadyInitialized = errors.New("already initialized")

// ErrDuplicateVerifiedIdentity is returned by CreateSource when a source with
// the same (connector_id, verified_external_id) already exists.
var ErrDuplicateVerifiedIdentity = errors.New("source with the same verified identity already exists")

// User represents a local ABCMovies user (e.g. the initial admin).
// user_id is the stable unique identifier; username is a login/display name.
type User struct {
	UserID   string
	Username string
	Hash     string
	Created  time.Time
}

// Session represents a logged-in session owned by a user.
type Session struct {
	ID      string
	UserID  string
	Created time.Time
	Expires time.Time
}

// Source represents a connected source (non-sensitive). Never holds credentials.
// source_id is the opaque internal primary key; (connector_id, verified_external_id)
// is the verified-identity dedup key (DEC-001, Build 1 instance scope).
type Source struct {
	SourceID           string
	ConnectorID        string
	VerifiedExternalID string
	Endpoint           string
	DisplayName        string
	Status             int32  // values aligned to abcmovies.source.v1.SourceStatus; mapped in core
	StatusDetail       string // safe, actionable; never a secret
	CreatedBy          string
	Created            time.Time
	Updated            time.Time
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    user_id   TEXT PRIMARY KEY,
    username  TEXT NOT NULL UNIQUE,
    hash      TEXT NOT NULL,
    created   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS sessions (
    id        TEXT PRIMARY KEY,
    user_id   TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires   TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS library_titles (
    title_id            TEXT PRIMARY KEY,
    source_id           TEXT NOT NULL REFERENCES sources(source_id) ON DELETE CASCADE,
    external_id         TEXT NOT NULL,
    name                TEXT NOT NULL,
    kind                INTEGER NOT NULL,
    year                INTEGER NOT NULL DEFAULT 0,
    overview            TEXT NOT NULL DEFAULT '',
    availability        INTEGER NOT NULL,
    availability_detail TEXT NOT NULL DEFAULT '',
    created             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_id, external_id)
);
CREATE INDEX IF NOT EXISTS library_titles_name ON library_titles(name);
CREATE INDEX IF NOT EXISTS library_titles_source_name ON library_titles(source_id, name);
CREATE TABLE IF NOT EXISTS source_library_sync_states (
    source_id      TEXT PRIMARY KEY REFERENCES sources(source_id) ON DELETE CASCADE,
    last_outcome   INTEGER NOT NULL,
    last_synced_at TIMESTAMP,
    detail         TEXT NOT NULL DEFAULT '',
    title_count    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS sessions_expires ON sessions(expires);
CREATE TABLE IF NOT EXISTS sources (
    source_id            TEXT PRIMARY KEY,
    connector_id         TEXT NOT NULL,
    verified_external_id TEXT NOT NULL,
    endpoint             TEXT NOT NULL,
    display_name         TEXT NOT NULL,
    status               INTEGER NOT NULL,
    status_detail        TEXT NOT NULL DEFAULT '',
    created_by           TEXT NOT NULL,
    created              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated              TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(connector_id, verified_external_id)
);
CREATE INDEX IF NOT EXISTS sources_connector_verified
    ON sources(connector_id, verified_external_id);
`

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (and migrates) the store at path.

// Store wraps the SQLite database.
// Open opens (and migrates) the store at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite supports a single writer; serializing connections avoids SQLITE_BUSY
	// under concurrent access.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	s := &Store{db: db}
	// Best-effort cleanup of expired sessions on startup.
	_, _ = s.DeleteExpiredSessions(context.Background())
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// HasUser reports whether any user exists (i.e. the instance is initialized).

// isUniqueConstraint reports whether err is an SQLite UNIQUE/PRIMARY KEY
// constraint violation. modernc sqlite reports extended code 2067
// (SQLITE_CONSTRAINT_UNIQUE) / 1555; match the message text as well so the
// checks stay robust across driver versions.
func isUniqueConstraint(err error) bool {
	var serr *sqlite.Error
	if errors.As(err, &serr) {
		return serr.Code() == 2067 || serr.Code() == 1555 || serr.Code()&0xff == 19
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "2067") || strings.Contains(msg, "1555")
}
