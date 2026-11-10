package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Proto-free values aligned to abcmovies.library.v1 enums. Core maps these to
// proto enums; the store never imports generated contracts.
const (
	LibraryOutcomeSuccess int32 = 1
	LibraryOutcomePartial int32 = 2
	LibraryOutcomeFailed  int32 = 3

	LibraryTitleKindMovie  int32 = 1
	LibraryTitleKindSeries int32 = 2
	LibraryTitleKindOther  int32 = 3

	LibraryAvailabilityAvailable   int32 = 1
	LibraryAvailabilityUnavailable int32 = 2
)

// LibraryTitle represents a non-sensitive library title from a source snapshot.
// Never holds credentials or linked access secrets.
// title_id is the opaque internal primary key; (source_id, external_id) is the
// upstream identity used to keep title_id stable across syncs.
type LibraryTitle struct {
	TitleID            string
	SourceID           string
	SourceDisplayName  string
	ExternalID         string
	Name               string
	Kind               int32
	Year               int32
	Overview           string
	Availability       int32
	AvailabilityDetail string
	Created            time.Time
	Updated            time.Time
}

// LibraryTitleInput is the non-sensitive data for a title seen in a source
// snapshot. Availability is not carried: the store derives it from the sync
// outcome (a title seen in a successful snapshot is available; missing titles
// become unavailable).
type LibraryTitleInput struct {
	ExternalID string
	Name       string
	Kind       int32
	Year       int32
	Overview   string
}

// LibrarySyncState is the per-source sync/freshness state recorded after each
// sync (safe, actionable; never a secret).
type LibrarySyncState struct {
	SourceID          string
	SourceDisplayName string
	LastOutcome       int32
	LastSyncedAt      *time.Time
	Detail            string
	TitleCount        int32
}

// LibraryListResult is a page of titles plus the per-source freshness states.
type LibraryListResult struct {
	Titles     []*LibraryTitle
	SyncStates []*LibrarySyncState
	TotalCount int32
}

// ReplaceSourceSnapshot applies a completed sync against one source atomically:
//   - it upserts every title in items (keyed by (source_id, external_id));
//   - on a successful sync, any title of this source that is absent from items
//     is marked unavailable (never deleted), preserving history;
//   - on a partial sync, only the seen titles are upserted and absent titles are
//     left unchanged (insufficient evidence);
//   - on a failed sync, no title rows are changed;
//   - the per-source sync state is upserted in the same transaction.
func (s *Store) ReplaceSourceSnapshot(ctx context.Context, sourceID string, outcome int32, syncedAt *time.Time, detail string, items []LibraryTitleInput) (*LibrarySyncState, error) {
	if outcome != LibraryOutcomeSuccess && outcome != LibraryOutcomePartial && outcome != LibraryOutcomeFailed {
		return nil, fmt.Errorf("invalid library sync outcome %d", outcome)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("replace snapshot: begin: %w", err)
	}

	var sourceName string
	if err := tx.QueryRowContext(ctx, `SELECT display_name FROM sources WHERE source_id = ?`, sourceID).Scan(&sourceName); err != nil {
		_ = tx.Rollback()
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read source: %w", err)
	}

	now := time.Now().UTC()
	if outcome != LibraryOutcomeFailed {
		for _, item := range items {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				_ = tx.Rollback()
				return nil, fmt.Errorf("title with external_id %q has an empty name", item.ExternalID)
			}
			externalID := strings.TrimSpace(item.ExternalID)
			if externalID == "" {
				_ = tx.Rollback()
				return nil, fmt.Errorf("title %q has an empty external_id", item.Name)
			}
			if item.Kind != LibraryTitleKindMovie && item.Kind != LibraryTitleKindSeries && item.Kind != LibraryTitleKindOther {
				_ = tx.Rollback()
				return nil, fmt.Errorf("title %q has an unsupported kind %d", externalID, item.Kind)
			}

			var titleID string
			err := tx.QueryRowContext(ctx,
				`SELECT title_id FROM library_titles WHERE source_id = ? AND external_id = ?`, sourceID, externalID).Scan(&titleID)
			switch {
			case err == sql.ErrNoRows:
				id := make([]byte, 16)
				if _, e := rand.Read(id); e != nil {
					_ = tx.Rollback()
					return nil, fmt.Errorf("generate title id: %w", e)
				}
				titleID = "ttl_" + hex.EncodeToString(id)
				if _, e := tx.ExecContext(ctx,
					`INSERT INTO library_titles
					    (title_id, source_id, external_id, name, kind, year, overview, availability, availability_detail, created, updated)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					titleID, sourceID, externalID, name, item.Kind, item.Year, item.Overview,
					LibraryAvailabilityAvailable, "", now, now); e != nil {
					_ = tx.Rollback()
					return nil, fmt.Errorf("insert title %q: %w", externalID, e)
				}
			case err != nil:
				_ = tx.Rollback()
				return nil, fmt.Errorf("read existing title %q: %w", externalID, err)
			default:
				if _, e := tx.ExecContext(ctx,
					`UPDATE library_titles
					   SET name = ?, kind = ?, year = ?, overview = ?, availability = ?, availability_detail = ?, updated = ?
					 WHERE title_id = ?`,
					name, item.Kind, item.Year, item.Overview, LibraryAvailabilityAvailable, "", now, titleID); e != nil {
					_ = tx.Rollback()
					return nil, fmt.Errorf("update title %q: %w", externalID, e)
				}
			}
		}
	}

	// Successful snapshot: absent rows are unavailable (never deleted).
	if outcome == LibraryOutcomeSuccess {
		if _, err := tx.ExecContext(ctx,
			`CREATE TEMPORARY TABLE IF NOT EXISTS library_sync_seen (
				external_id TEXT PRIMARY KEY
			)`); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("sync seen temp table: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_sync_seen`); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("clear sync seen: %w", err)
		}
		for _, item := range items {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO library_sync_seen (external_id) VALUES (?)`, strings.TrimSpace(item.ExternalID)); err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("record seen title %q: %w", item.ExternalID, err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE library_titles
			   SET availability = ?, availability_detail = ?, updated = ?
			 WHERE source_id = ?
			   AND availability != ?
			   AND external_id NOT IN (SELECT external_id FROM library_sync_seen)`,
			LibraryAvailabilityUnavailable, "the source no longer provides this title", now, sourceID, LibraryAvailabilityUnavailable); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("mark absent titles unavailable: %w", err)
		}
	}

	// Recount availability for the source in the same transaction so the sync
	// state never claims a misleading count.
	var available int32
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM library_titles WHERE source_id = ? AND availability = ?`,
		sourceID, LibraryAvailabilityAvailable).Scan(&available); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("count titles: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO source_library_sync_states (source_id, last_outcome, last_synced_at, detail, title_count)
		     VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(source_id) DO UPDATE SET
		     last_outcome  = excluded.last_outcome,
		     last_synced_at = excluded.last_synced_at,
		     detail        = excluded.detail,
		     title_count   = excluded.title_count`,
		sourceID, outcome, syncedAt, strings.TrimSpace(detail), available); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("upsert sync state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit snapshot: %w", err)
	}

	return s.getSourceSyncState(ctx, sourceID)
}

func (s *Store) getSourceSyncState(ctx context.Context, sourceID string) (*LibrarySyncState, error) {
	st := &LibrarySyncState{}
	var syncedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx,
		`SELECT s.source_id, s.last_outcome, s.last_synced_at, s.detail, s.title_count, r.display_name
		   FROM source_library_sync_states s
		   JOIN sources r ON r.source_id = s.source_id
		  WHERE s.source_id = ?`,
		sourceID).Scan(&st.SourceID, &st.LastOutcome, &syncedAt, &st.Detail, &st.TitleCount, &st.SourceDisplayName); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get sync state: %w", err)
	}
	if syncedAt.Valid {
		t := syncedAt.Time.UTC()
		st.LastSyncedAt = &t
	}
	return st, nil
}

// ListSyncStates returns one freshness state for every connected source,
// defaulting to a never-synced state for sources that have not synced yet.
func (s *Store) ListSyncStates(ctx context.Context) ([]*LibrarySyncState, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.source_id, r.display_name,
		        COALESCE(s.last_outcome, 0), s.last_synced_at, COALESCE(s.detail, ''), COALESCE(s.title_count, 0)
		   FROM sources r
		   LEFT JOIN source_library_sync_states s ON s.source_id = r.source_id
		  ORDER BY r.created DESC, r.source_id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list sync states: %w", err)
	}
	defer rows.Close()
	out := []*LibrarySyncState{}
	for rows.Next() {
		st := &LibrarySyncState{}
		var syncedAt sql.NullTime
		if err := rows.Scan(&st.SourceID, &st.SourceDisplayName, &st.LastOutcome, &syncedAt, &st.Detail, &st.TitleCount); err != nil {
			return nil, fmt.Errorf("scan sync state: %w", err)
		}
		if syncedAt.Valid {
			t := syncedAt.Time.UTC()
			st.LastSyncedAt = &t
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func clampLimit(limit int32) int {
	switch {
	case limit <= 0:
		return 50
	case limit > 200:
		return 200
	default:
		return int(limit)
	}
}

func clampOffset(offset int32) int {
	if offset < 0 {
		return 0
	}
	return int(offset)
}

// GetTitleByExternal returns the title for (source_id, external_id), or ErrNotFound.
func (s *Store) GetTitleByExternal(ctx context.Context, sourceID, externalID string) (*LibraryTitle, error) {
	t := &LibraryTitle{}
	err := s.db.QueryRowContext(ctx,
		`SELECT t.title_id, t.source_id, r.display_name, t.external_id, t.name, t.kind, t.year, t.overview,
		        t.availability, t.availability_detail, t.created, t.updated
		   FROM library_titles t
		   JOIN sources r ON r.source_id = t.source_id
		  WHERE t.source_id = ? AND t.external_id = ?`, sourceID, externalID).Scan(&t.TitleID, &t.SourceID, &t.SourceDisplayName, &t.ExternalID, &t.Name, &t.Kind, &t.Year, &t.Overview, &t.Availability, &t.AvailabilityDetail, &t.Created, &t.Updated)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get title by external id: %w", err)
	}
	return t, nil
}

// ListTitles returns a page of titles ordered by name. Available and older
// unavailable titles are both returned so honesty signals stay visible to the
// interface; the availability state is carried on each title.
func (s *Store) ListTitles(ctx context.Context, limit, offset int32) (*LibraryListResult, error) {
	return s.queryTitles(ctx, "", limit, offset)
}

// SearchTitles returns a page of titles whose names contain query
// (case-insensitive, with LIKE metacharacters escaped).
func (s *Store) SearchTitles(ctx context.Context, query string, limit, offset int32) (*LibraryListResult, error) {
	return s.queryTitles(ctx, strings.TrimSpace(query), limit, offset)
}

func (s *Store) queryTitles(ctx context.Context, query string, limit, offset int32) (*LibraryListResult, error) {
	lmt := clampLimit(limit)
	off := clampOffset(offset)

	where := ""
	args := []any{}
	countWhere := ""
	if query != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
		where = ` WHERE LOWER(t.name) LIKE LOWER(?) ESCAPE '\'`
		args = append(args, `%`+escaped+`%`)
		countWhere = ` WHERE LOWER(name) LIKE LOWER(?) ESCAPE '\'`
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT t.title_id, t.source_id, r.display_name, t.external_id, t.name, t.kind, t.year, t.overview,
		        t.availability, t.availability_detail, t.created, t.updated
		   FROM library_titles t
		   JOIN sources r ON r.source_id = t.source_id`+where+`
		  ORDER BY LOWER(t.name), t.title_id
		  LIMIT ? OFFSET ?`, append(args, lmt, off)...)
	if err != nil {
		return nil, fmt.Errorf("list titles: %w", err)
	}
	defer rows.Close()
	titles := []*LibraryTitle{}
	for rows.Next() {
		t := &LibraryTitle{}
		if err := rows.Scan(&t.TitleID, &t.SourceID, &t.SourceDisplayName, &t.ExternalID, &t.Name, &t.Kind, &t.Year, &t.Overview, &t.Availability, &t.AvailabilityDetail, &t.Created, &t.Updated); err != nil {
			return nil, fmt.Errorf("scan title: %w", err)
		}
		titles = append(titles, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list titles: %w", err)
	}

	var total int32
	countArgs := args
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM library_titles`+countWhere, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count titles: %w", err)
	}

	syncStates, err := s.ListSyncStates(ctx)
	if err != nil {
		return nil, err
	}
	return &LibraryListResult{Titles: titles, SyncStates: syncStates, TotalCount: total}, nil
}

// GetTitle returns one title, or ErrNotFound if unknown.
func (s *Store) GetTitle(ctx context.Context, titleID string) (*LibraryTitle, error) {
	t := &LibraryTitle{}
	err := s.db.QueryRowContext(ctx,
		`SELECT t.title_id, t.source_id, r.display_name, t.external_id, t.name, t.kind, t.year, t.overview,
		        t.availability, t.availability_detail, t.created, t.updated
		   FROM library_titles t
		   JOIN sources r ON r.source_id = t.source_id
		  WHERE t.title_id = ?`, titleID).Scan(&t.TitleID, &t.SourceID, &t.SourceDisplayName, &t.ExternalID, &t.Name, &t.Kind, &t.Year, &t.Overview, &t.Availability, &t.AvailabilityDetail, &t.Created, &t.Updated)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get title: %w", err)
	}
	return t, nil
}
