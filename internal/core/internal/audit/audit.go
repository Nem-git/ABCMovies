// Package audit provides a minimal audit/event log. Entries contain no
// secrets, hashes, passwords, or session material.
package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event is a single audit/event log entry. Only non-sensitive identifiers
// are recorded; never credentials or session material.
type Event struct {
	Event    string `json:"event"`
	UserID   string `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
	Detail   string `json:"detail,omitempty"`
	TS       string `json:"ts"`
}

// Logger appends events to a file as JSON lines. The file is opened with
// owner-only permissions.
type Logger struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// DefaultPath resolves the audit log path for a database: the
// ABCMOVIES_AUDIT_LOG override wins, otherwise the log lives next to the
// database. The admin recovery tool and the core use the same convention.
func DefaultPath(dbPath string) string {
	if p := os.Getenv("ABCMOVIES_AUDIT_LOG"); p != "" {
		return p
	}
	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		return "abcmovies-audit.log"
	}
	return filepath.Join(dir, "abcmovies-audit.log")
}

// Open opens (appending to) the audit log at path with owner-only permissions.
func Open(path string) (*Logger, error) {
	if path == "" {
		return nil, errors.New("audit log path is required")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	return &Logger{path: path, file: f}, nil
}

// Close closes the underlying file.
func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	return l.file.Close()
}

// Path returns the log path.
func (l *Logger) Path() string { return l.path }

// Log appends event with the current UTC timestamp.
func (l *Logger) Log(event Event) error {
	if event.Event == "" {
		return errors.New("audit event name is required")
	}
	event.TS = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.file.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	_ = l.file.Sync()
	return nil
}
