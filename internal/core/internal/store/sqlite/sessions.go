package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

func (s *Store) CreateSession(ctx context.Context, userID string, ttl time.Duration) (*Session, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	now := time.Now().UTC()
	sess := &Session{
		ID:      hex.EncodeToString(token),
		UserID:  userID,
		Created: now,
		Expires: now.Add(ttl),
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, created, expires) VALUES (?, ?, ?, ?)`,
		sess.ID, sess.UserID, sess.Created, sess.Expires); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return sess, nil
}

// ValidateSession returns the session if it exists and has not expired, or ErrNotFound.

func (s *Store) ValidateSession(ctx context.Context, id string) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, created, expires FROM sessions WHERE id = ? AND expires > ?`, id, time.Now().UTC()).Scan(&sess.ID, &sess.UserID, &sess.Created, &sess.Expires)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("validate session: %w", err)
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

// GetAdminCredentials returns the user_id and username of the initial admin
// (the first user), or ErrNotFound when the instance is not initialized.

func (s *Store) GetAdminCredentials(ctx context.Context) (*User, error) {
	u := &User{}
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, username, hash, created FROM users ORDER BY created ASC, user_id ASC LIMIT 1`).Scan(&u.UserID, &u.Username, &u.Hash, &u.Created)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get admin credentials: %w", err)
	}
	return u, nil
}

// SetPassword updates the given user's password hash and invalidates all of
// that user's sessions. It returns the number of sessions invalidated.

func (s *Store) SetPassword(ctx context.Context, userID, hash string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("set password: begin: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET hash = ? WHERE user_id = ?`, hash, userID); err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("set password: update hash: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("set password: delete sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("set password: commit: %w", err)
	}
	return res.RowsAffected()
}

// DeleteExpiredSessions removes sessions whose expiry has passed. It is
// best-effort hygiene; validation already ignores expired sessions.

func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires <= ?`, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}

// GetSourceByVerifiedIdentity returns the source for (connector_id,
// verified_external_id), or ErrNotFound. Used for duplicate detection (DEC-001).
// Build 1 scope is instance-level (household/operator); future public/multi-user
// dedup visibility must be scoped by a follow-up decision.
