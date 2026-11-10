package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

// maxListLimit caps page sizes on browse/search.
const maxListLimit = 200

// maxSyncDetailLength bounds safe sync-detail strings stored and displayed.
const maxSyncDetailLength = 256

// LibraryService implements libraryv1.LibraryServiceServer.
type LibraryService struct {
	libraryv1.UnimplementedLibraryServiceServer

	store      *store.Store
	fetcher    libraryFetcher
	authorizer playbackAuthorizer
	session    *SessionState
}

// libraryFetcher is the dependency used to pull a library snapshot from a
// source during sync. Tests inject a fake; production uses the Jellyfin
// connector (NewLibraryService wires it).
type libraryFetcher interface {
	FetchLibrary(ctx context.Context, endpoint string, creds jellyfin.Credentials) (*jellyfin.LibrarySnapshot, *jellyfin.Error)
}

// NewLibraryService returns a LibraryService backed by the store and the
// Jellyfin connector. sv is the shared in-memory session state (core wires
// one instance for all services).
func NewLibraryService(s *store.Store, sv *SessionState) *LibraryService {
	return NewLibraryServiceWithFetcher(s, jellyfin.NewClient(), sv)
}

// NewLibraryServiceWithFetcher returns a LibraryService with a chosen library
// fetcher. Tests use it to substitute a fake. sv is the shared in-memory
// session state.
func NewLibraryServiceWithFetcher(s *store.Store, fetcher libraryFetcher, sv *SessionState) *LibraryService {
	return &LibraryService{
		store:      s,
		fetcher:    fetcher,
		authorizer: jellyfin.NewClient(),
		session:    sv,
	}
}

// SyncSource refreshes a source's library snapshot using transient credentials.
func (l *LibraryService) SyncSource(ctx context.Context, req *libraryv1.SyncSourceRequest) (*libraryv1.SyncSourceResponse, error) {
	actor := strings.TrimSpace(req.GetActorUserId())
	if actor == "" {
		return nil, status.Errorf(codes.InvalidArgument, "actor_user_id is required")
	}
	sourceID := strings.TrimSpace(req.GetSourceId())
	if sourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "source_id is required")
	}
	src, err := l.store.GetSource(ctx, sourceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "source not found")
		}
		return nil, status.Errorf(codes.Internal, "read source: %v", err)
	}
	if strings.TrimSpace(src.ConnectorID) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "source has no connector")
	}

	// Credentials: supplied in the request (then cached in memory for the
	// session), or omitted to reuse this session's cache. Nothing is
	// persisted; if neither is available the call fails with
	// FailedPrecondition.
	creds, err := l.resolveCredsFromOneof(actor, req.GetPassword(), req.GetApiKey())
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	snapshot, fetchErr := l.fetcher.FetchLibrary(ctx, src.Endpoint, creds)
	if fetchErr != nil {
		detail := safeSyncDetail(fetchErr.Message)
		if _, e := l.store.ReplaceSourceSnapshot(ctx, sourceID, store.LibraryOutcomeFailed, &now, detail, nil); e != nil {
			return nil, status.Errorf(codes.Internal, "record sync state: %v", e)
		}
		return nil, mapVerifyError(fetchErr)
	}

	// Auth/fetch succeeded: cache the credentials for this session.
	l.session.creds.Put(actor, creds)

	items := make([]store.LibraryTitleInput, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		items = append(items, store.LibraryTitleInput{
			ExternalID: item.ExternalID,
			Name:       item.Name,
			Kind:       titleKindFromString(item.Kind),
			Year:       item.Year,
			Overview:   item.Overview,
		})
	}

	outcome := store.LibraryOutcomeSuccess
	detail := ""
	if snapshot.Skipped > 0 {
		outcome = store.LibraryOutcomePartial
		detail = safeSyncDetail(fmt.Sprintf("skipped %d titles because of unusable upstream data; partial results shown", snapshot.Skipped))
	}
	syncState, err := l.store.ReplaceSourceSnapshot(ctx, sourceID, outcome, &now, detail, items)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply snapshot: %v", err)
	}
	return &libraryv1.SyncSourceResponse{
		SourceId: sourceID,
		Outcome:  syncOutcomeToProto(outcome),
		State:    syncStateToProto(syncState),
	}, nil
}

// ListTitles returns titles from the reachable library (browse).
func (l *LibraryService) ListTitles(ctx context.Context, req *libraryv1.ListTitlesRequest) (*libraryv1.ListTitlesResponse, error) {
	if err := actorRequired(req.GetActorUserId()); err != nil {
		return nil, err
	}
	result, err := l.store.ListTitles(ctx, clampListLimit(req.GetLimit()), clampOffset(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list titles: %v", err)
	}
	return &libraryv1.ListTitlesResponse{
		Titles:     titlesToProto(result.Titles),
		SyncStates: syncStatesToProto(result.SyncStates),
		TotalCount: result.TotalCount,
	}, nil
}

// SearchTitles returns titles matching a query across the reachable library.
func (l *LibraryService) SearchTitles(ctx context.Context, req *libraryv1.SearchTitlesRequest) (*libraryv1.SearchTitlesResponse, error) {
	if err := actorRequired(req.GetActorUserId()); err != nil {
		return nil, err
	}
	result, err := l.store.SearchTitles(ctx, strings.TrimSpace(req.GetQuery()), clampListLimit(req.GetLimit()), clampOffset(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search titles: %v", err)
	}
	return &libraryv1.SearchTitlesResponse{
		Titles:     titlesToProto(result.Titles),
		SyncStates: syncStatesToProto(result.SyncStates),
		TotalCount: result.TotalCount,
	}, nil
}

// GetTitle returns one title with detail and availability.
func (l *LibraryService) GetTitle(ctx context.Context, req *libraryv1.GetTitleRequest) (*libraryv1.GetTitleResponse, error) {
	if err := actorRequired(req.GetActorUserId()); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetTitleId())
	if id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "title_id is required")
	}
	src, err := l.store.GetTitle(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "title not found")
		}
		return nil, status.Errorf(codes.Internal, "get title: %v", err)
	}
	return &libraryv1.GetTitleResponse{Title: titleToProto(src)}, nil
}

func actorRequired(actor string) error {
	if strings.TrimSpace(actor) == "" {
		return status.Errorf(codes.InvalidArgument, "actor_user_id is required")
	}
	return nil
}

func clampListLimit(limit int32) int32 {
	switch {
	case limit <= 0:
		return 50
	case limit > maxListLimit:
		return maxListLimit
	default:
		return limit
	}
}

func clampOffset(offset int32) int32 {
	if offset < 0 {
		return 0
	}
	return offset
}



func titleKindFromString(kind string) int32 {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "movie":
		return store.LibraryTitleKindMovie
	case "series":
		return store.LibraryTitleKindSeries
	default:
		return store.LibraryTitleKindOther
	}
}

func syncOutcomeToProto(outcome int32) libraryv1.SyncOutcome {
	switch outcome {
	case store.LibraryOutcomeSuccess:
		return libraryv1.SyncOutcome_SYNC_OUTCOME_SUCCESS
	case store.LibraryOutcomePartial:
		return libraryv1.SyncOutcome_SYNC_OUTCOME_PARTIAL
	case store.LibraryOutcomeFailed:
		return libraryv1.SyncOutcome_SYNC_OUTCOME_FAILED
	default:
		return libraryv1.SyncOutcome_SYNC_OUTCOME_UNSPECIFIED
	}
}

func titleKindToProto(kind int32) libraryv1.TitleKind {
	switch kind {
	case store.LibraryTitleKindMovie:
		return libraryv1.TitleKind_TITLE_KIND_MOVIE
	case store.LibraryTitleKindSeries:
		return libraryv1.TitleKind_TITLE_KIND_SERIES
	case store.LibraryTitleKindOther:
		return libraryv1.TitleKind_TITLE_KIND_OTHER
	default:
		return libraryv1.TitleKind_TITLE_KIND_UNSPECIFIED
	}
}

func availabilityToProto(availability int32) libraryv1.AvailabilityState {
	switch availability {
	case store.LibraryAvailabilityAvailable:
		return libraryv1.AvailabilityState_AVAILABILITY_STATE_AVAILABLE
	case store.LibraryAvailabilityUnavailable:
		return libraryv1.AvailabilityState_AVAILABILITY_STATE_UNAVAILABLE
	default:
		return libraryv1.AvailabilityState_AVAILABILITY_STATE_UNSPECIFIED
	}
}

func syncStateToProto(src *store.LibrarySyncState) *libraryv1.SourceSyncState {
	if src == nil {
		return &libraryv1.SourceSyncState{}
	}
	out := &libraryv1.SourceSyncState{
		SourceId:          src.SourceID,
		SourceDisplayName: src.SourceDisplayName,
		LastOutcome:       syncOutcomeToProto(src.LastOutcome),
		Detail:            src.Detail,
		TitleCount:        src.TitleCount,
	}
	if src.LastSyncedAt != nil {
		out.LastSyncedAt = timestamppb.New(*src.LastSyncedAt)
	}
	return out
}

func syncStatesToProto(srcs []*store.LibrarySyncState) []*libraryv1.SourceSyncState {
	out := make([]*libraryv1.SourceSyncState, 0, len(srcs))
	for _, src := range srcs {
		out = append(out, syncStateToProto(src))
	}
	return out
}

func titleToProto(src *store.LibraryTitle) *libraryv1.Title {
	if src == nil {
		return &libraryv1.Title{}
	}
	kind := titleKindToProto(src.Kind)
	availability := availabilityToProto(src.Availability)
	return &libraryv1.Title{
		TitleId:            src.TitleID,
		SourceId:           src.SourceID,
		SourceDisplayName:  src.SourceDisplayName,
		ExternalId:         src.ExternalID,
		Name:               src.Name,
		Kind:               kind,
		Year:               src.Year,
		Overview:           src.Overview,
		Availability:       availability,
		AvailabilityDetail: src.AvailabilityDetail,
	}
}

func titlesToProto(srcs []*store.LibraryTitle) []*libraryv1.Title {
	out := make([]*libraryv1.Title, 0, len(srcs))
	for _, src := range srcs {
		out = append(out, titleToProto(src))
	}
	return out
}

func safeSyncDetail(message string) string {
	msg := strings.TrimSpace(message)
	if len(msg) > maxSyncDetailLength {
		msg = msg[:maxSyncDetailLength]
	}
	return msg
}
