// Package store implements the core store: Open/Close plus the public,
// proto-free domain surface. Domain files live under ./sqlite.
package store

import "github.com/Nem-git/ABCMovies/internal/core/internal/store/sqlite"

// Errors re-exported from the SQLite implementation for stable call sites.
var (
	ErrNotFound                  = sqlite.ErrNotFound
	ErrAlreadyInitialized        = sqlite.ErrAlreadyInitialized
	ErrDuplicateVerifiedIdentity = sqlite.ErrDuplicateVerifiedIdentity
)

type Store = sqlite.Store

func Open(path string) (*Store, error) { return sqlite.Open(path) }

// Types re-exported so call sites keep a single import path.
type User = sqlite.User
type Session = sqlite.Session
type Source = sqlite.Source
type LibraryTitle = sqlite.LibraryTitle
type LibraryTitleInput = sqlite.LibraryTitleInput
type LibrarySyncState = sqlite.LibrarySyncState
type LibraryListResult = sqlite.LibraryListResult

// Library proto-free mapped values re-exported for core.
const (
	LibraryOutcomeSuccess = sqlite.LibraryOutcomeSuccess
	LibraryOutcomePartial = sqlite.LibraryOutcomePartial
	LibraryOutcomeFailed  = sqlite.LibraryOutcomeFailed

	LibraryTitleKindMovie  = sqlite.LibraryTitleKindMovie
	LibraryTitleKindSeries = sqlite.LibraryTitleKindSeries
	LibraryTitleKindOther  = sqlite.LibraryTitleKindOther

	LibraryAvailabilityAvailable   = sqlite.LibraryAvailabilityAvailable
	LibraryAvailabilityUnavailable = sqlite.LibraryAvailabilityUnavailable
)
