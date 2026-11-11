package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Job kinds. Only sync exists today; downloads (Later) would add one.
const JobKindSync = "sync"

// Job states. Proto-free integers; the core maps these to nothing in Build 1
// (jobs are never exposed over gRPC in ticket A). `succeeded`, `partial`, and
// `failed` mirror LibraryOutcome* so a completed sync job and its source
// freshness state say the same thing.
const (
	JobStateRunning     int32 = 1
	JobStateSucceeded   int32 = 2
	JobStatePartial     int32 = 3
	JobStateFailed      int32 = 4
	JobStateInterrupted int32 = 5
)

const jobDetailMaxLength = 256

// Job is a unit of long-running work. It never holds credentials or linked
// access secrets; Detail is user-facing copy and must be safe/actionable.
// In-flight state is durable so the store can tell, at the next startup,
// whether a job died with the previous process.
type Job struct {
	JobID     string
	Kind      string
	UserID    string
	SourceID  string
	State     int32
	StartedAt time.Time
	UpdatedAt time.Time
	Detail    string
}

func newJobID() (string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", err
	}
	return "job_" + hex.EncodeToString(id), nil
}

// CreateJob records a new job in state running. The caller must complete it
// via CompleteJob; otherwise the job is swept to interrupted at the next
// startup. Failure to create a job means the caller should not start the
// work — an untracked long-running operation is worse than a rejected one.
func (s *Store) CreateJob(ctx context.Context, kind, userID, sourceID string) (*Job, error) {
	kind = strings.TrimSpace(kind)
	userID = strings.TrimSpace(userID)
	sourceID = strings.TrimSpace(sourceID)
	if kind == "" || userID == "" || sourceID == "" {
		return nil, fmt.Errorf("kind, user_id and source_id are required")
	}
	if kind != JobKindSync {
		return nil, fmt.Errorf("unsupported job kind %q", kind)
	}
	now := time.Now().UTC()
	for attempts := 0; attempts < 3; attempts++ {
		id, err := newJobID()
		if err != nil {
			return nil, fmt.Errorf("generate job id: %w", err)
		}
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO jobs (job_id, kind, user_id, source_id, state, started_at, updated_at, detail)
			 VALUES (?, ?, ?, ?, ?, ?, ?, '')`,
			id, kind, userID, sourceID, JobStateRunning, now, now)
		if err == nil {
			return &Job{JobID: id, Kind: kind, UserID: userID, SourceID: sourceID, State: JobStateRunning, StartedAt: now, UpdatedAt: now}, nil
		}
		if !isUniqueConstraint(err) {
			return nil, fmt.Errorf("insert job: %w", err)
		}
	}
	return nil, fmt.Errorf("insert job: exhausted id retries")
}

// CompleteJob records the terminal state of a job. detail is trimmed and
// hard-truncated (it is user-facing copy; callers pass already-safe strings).
func (s *Store) CompleteJob(ctx context.Context, jobID string, state int32, detail string) error {
	switch state {
	case JobStateSucceeded, JobStatePartial, JobStateFailed, JobStateInterrupted:
	default:
		return fmt.Errorf("invalid terminal job state %d", state)
	}
	detail = strings.TrimSpace(detail)
	if len(detail) > jobDetailMaxLength {
		detail = detail[:jobDetailMaxLength]
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET state = ?, updated_at = ?, detail = ? WHERE job_id = ?`,
		state, time.Now().UTC(), detail, jobID)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

// LatestSyncJob returns the most recent sync job for a source, or ErrNotFound
// if none exists. Used by tests and (later) by the status surface for ticket B.
func (s *Store) LatestSyncJob(ctx context.Context, sourceID string) (*Job, error) {
	j := &Job{}
	if err := s.db.QueryRowContext(ctx,
		`SELECT job_id, kind, user_id, source_id, state, started_at, updated_at, detail
		   FROM jobs
		  WHERE kind = ? AND source_id = ?
		  ORDER BY started_at DESC, job_id DESC
		  LIMIT 1`, JobKindSync, strings.TrimSpace(sourceID)).
		Scan(&j.JobID, &j.Kind, &j.UserID, &j.SourceID, &j.State, &j.StartedAt, &j.UpdatedAt, &j.Detail); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("latest sync job: %w", err)
	}
	return j, nil
}

// interruptOrphanedDetail is the safe, actionable copy recorded on jobs swept
// at startup. The prior library snapshot is retained in every interruption
// path (ADR 0004), so the disclose note is always true in Build 1.
const interruptOrphanedDetail = "the instance restarted before this job finished; the prior library snapshot was kept"

// InterruptOrphanedJobs sweeps jobs left in state running to interrupted.
// Jobs run in-process, so every running job at startup is an orphan by
// definition, for any kind of job. In the same transaction it moves each
// affected source's sync state to outcome interrupted — a sync that never
// finished must not leave a fresh "up to date" freshness signal (title
// count and last_synced_at are preserved). Sources whose jobs were orphaned
// keep their titles; nothing is deleted.
//
// Best-effort, like the expired-session cleanup in Open: the instance still
// starts if the sweep fails.
func (s *Store) InterruptOrphanedJobs(ctx context.Context, detail string) (int64, error) {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		detail = interruptOrphanedDetail
	}
	if len(detail) > jobDetailMaxLength {
		detail = detail[:jobDetailMaxLength]
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("interrupt orphaned jobs: begin: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT job_id, kind, user_id, source_id FROM jobs WHERE state = ?`, JobStateRunning)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("interrupt orphaned jobs: read: %w", err)
	}
	orphans := []*Job{}
	for rows.Next() {
		j := &Job{}
		if err := rows.Scan(&j.JobID, &j.Kind, &j.UserID, &j.SourceID); err != nil {
			_ = rows.Close()
			_ = tx.Rollback()
			return 0, fmt.Errorf("interrupt orphaned jobs: scan: %w", err)
		}
		orphans = append(orphans, j)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("interrupt orphaned jobs: rows: %w", err)
	}
	if len(orphans) == 0 {
		return 0, tx.Commit()
	}
	now := time.Now().UTC()
	for _, j := range orphans {
		if _, err := tx.ExecContext(ctx,
			`UPDATE jobs SET state = ?, updated_at = ?, detail = ? WHERE job_id = ?`,
			JobStateInterrupted, now, detail, j.JobID); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("interrupt orphaned jobs: mark %q: %w", j.JobID, err)
		}
		if j.Kind != JobKindSync {
			continue
		}
		// Upsert keeps title_count and last_synced_at on conflict; a never-synced
		// source gets an honest interrupted marker with 0 titles.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO source_library_sync_states (source_id, last_outcome, last_synced_at, detail, title_count)
			     SELECT source_id, ?, NULL, ?, 0 FROM sources WHERE source_id = ?
			       ON CONFLICT(source_id) DO UPDATE SET last_outcome = excluded.last_outcome, detail = excluded.detail`,
			LibraryOutcomeInterrupted, detail, j.SourceID); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("interrupt orphaned jobs: mark sync state for %q: %w", j.SourceID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("interrupt orphaned jobs: commit: %w", err)
	}
	return int64(len(orphans)), nil
}
