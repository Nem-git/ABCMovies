package apiserver

import (
	"context"
	"strconv"
	"strings"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LibrarySeam is the slice of the library service the API layer calls
// (PLAN.md §5, §7.5): deriving the caller's library for reads, resolving
// cached display metadata, and answering account-reach questions for the
// accounts view and removal. One concrete implementation is the library
// Service; tests may arm a stub.
type LibrarySeam interface {
	Library(ctx context.Context, userID string) ([]*corev1.LibraryEntry, error)
	Metadata(ctx context.Context, ref string) (*corev1.TitleMetadata, bool, error)
	ReachAuthorized(accountID, userID string) (library.Reach, bool)
	ReachesForUser(userID string) []library.Reach
	RemoveReach(accountID string)
}

// SetLibrary arms the library seam used by every library and account RPC.
func (s *Server) SetLibrary(ls LibrarySeam) {
	s.library = ls
}

// libraryPageSize bounds a GetLibrary page. Concrete values live once: this is
// the P6 home before the number is recorded in TECHNICAL-DECISIONS.md.
const libraryPageSize = 100

// GetLibrary returns a page of the caller's merged library (PLAN.md §5, §8.1).
// Derivation, identity merging, and per-user caching all happen in the library
// service; this handler filters and paginates the derived view. The query
// surface matches the derived display surface (see the search-surface decision
// in TECHNICAL-DECISIONS.md; pagination uses a fixed page bound, recorded for
// the P6 docs pass).
func (s *Server) GetLibrary(ctx context.Context, req *apiv1.GetLibraryRequest) (*apiv1.GetLibraryResponse, error) {
	if err := schema.ValidateGetLibraryRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.library == nil {
		return nil, status.Error(codes.Unavailable, "library engine not configured")
	}
	uid, _ := UserIDFromContext(ctx)
	entries, err := s.library.Library(ctx, uid)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	// The page token is the opaque start index of the next page. Validating
	// it here keeps a hostile token from reaching the paging logic.
	start := 0
	if t := req.GetPageToken(); t != "" {
		n, err := strconv.Atoi(t)
		if err != nil || n < 0 || n > len(entries) {
			return nil, status.Error(codes.InvalidArgument, "invalid page token")
		}
		start = n
	}
	q := strings.ToLower(strings.TrimSpace(req.GetQuery()))
	if q != "" {
		var filtered []*corev1.LibraryEntry
		for _, e := range entries {
			if entryMatchesQuery(ctx, s.library, e, q) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	end := min(start+libraryPageSize, len(entries))
	items := make([]*apiv1.LibraryItem, 0, end-start)
	for _, e := range entries[start:end] {
		meta, _, mErr := s.library.Metadata(ctx, e.GetMetadataRef())
		if mErr != nil {
			// Display metadata is best-effort; one cache hiccup must not
			// thin the caller's library view.
			continue
		}
		items = append(items, &apiv1.LibraryItem{Entry: e, Metadata: meta})
	}
	next := ""
	if end < len(entries) {
		next = strconv.Itoa(end)
	}
	return &apiv1.GetLibraryResponse{Items: items, NextPageToken: next}, nil
}

// entryMatchesQuery reports whether an entry matches the free-text query over
// its derived display surface: the cached title when present and, failing
// that, the entry's id and every corroborated external identity value. The
// matching is deliberately substring and case-insensitive — frontends own
// richer search and surface autocomplete over this; the core stays
// deterministic and cheap (search-surface decision, TECHNICAL-DECISIONS.md).
func entryMatchesQuery(ctx context.Context, ls LibrarySeam, e *corev1.LibraryEntry, q string) bool {
	if strings.Contains(strings.ToLower(e.GetId()), q) {
		return true
	}
	for _, id := range e.GetExternalIdentities() {
		if id == nil {
			continue
		}
		if strings.Contains(strings.ToLower(id.GetValue()), q) ||
			strings.Contains(strings.ToLower(id.GetNamespace()+":"+id.GetValue()), q) {
			return true
		}
	}
	meta, ok, err := ls.Metadata(ctx, e.GetMetadataRef())
	if err != nil || !ok {
		// No cached record (or a hiccup): the title cannot be matched; the
		// entry still surfaces through its id/identity values above.
		return false
	}
	return strings.Contains(strings.ToLower(meta.GetTitle()), q)
}
