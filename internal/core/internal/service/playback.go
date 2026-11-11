package service

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

// playbackAuthorizer is the dependency used to authenticate against a source
// at ticket-mint time. Tests inject a fake; production uses the Jellyfin
// connector (NewLibraryServiceWithFetcher wires it).
type playbackAuthorizer interface {
	Authorize(ctx context.Context, endpoint string, creds jellyfin.Credentials) (string, *jellyfin.Error)
}

// CreatePlayTicket implements the playback ticket mint. See the proto for
// the contract.
func (l *LibraryService) CreatePlayTicket(ctx context.Context, req *libraryv1.CreatePlayTicketRequest) (*libraryv1.CreatePlayTicketResponse, error) {
	actor := strings.TrimSpace(req.GetActorUserId())
	if actor == "" {
		return nil, status.Errorf(codes.InvalidArgument, "actor_user_id is required")
	}
	titleID := strings.TrimSpace(req.GetTitleId())
	if titleID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "title_id is required")
	}
	t, err := l.store.GetTitle(ctx, titleID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "title not found")
		}
		return nil, status.Errorf(codes.Internal, "get title: %v", err)
	}
	if t.Availability != store.LibraryAvailabilityAvailable {
		return nil, status.Errorf(codes.FailedPrecondition, "this title is not currently presented as playable — try syncing again")
	}
	if t.Kind == store.LibraryTitleKindSeries {
		return nil, status.Errorf(codes.FailedPrecondition, "series playback is not supported in Build 1 yet — only movies can be played")
	}
	src, err := l.store.GetSource(ctx, t.SourceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "source not found")
		}
		return nil, status.Errorf(codes.Internal, "read source: %v", err)
	}

	creds, err := l.resolveCredsFromOneof(actor, req.GetPassword(), req.GetApiKey())
	if err != nil {
		return nil, err
	}

	authz, authErr := l.authorizer.Authorize(ctx, src.Endpoint, creds)
	if authErr != nil {
		return nil, mapVerifyError(authErr)
	}
	// Auth succeeded: cache the credentials for this session.
	l.session.creds.Put(actor, creds)

	streamURL, urlErr := jellyfin.StreamURL(src.Endpoint, t.ExternalID)
	if urlErr != nil {
		return nil, status.Errorf(codes.Internal, "build stream URL: %v", urlErr.Message)
	}

	expiresAt := l.session.tickets.now().Add(ticketIdleTTL)
	id, err := l.session.tickets.mint(playTicket{
		actorUserID:   actor,
		titleID:       titleID,
		sourceID:      src.SourceID,
		streamURL:     streamURL,
		authorization: authz,
		expiresAt:     expiresAt,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mint ticket: %v", err)
	}
	return &libraryv1.CreatePlayTicketResponse{Ticket: id, ExpiresAt: timestamppb.New(expiresAt)}, nil
}

// ResolvePlayTicket implements server-to-server ticket validation. See the
// proto for the contract.
func (l *LibraryService) ResolvePlayTicket(ctx context.Context, req *libraryv1.ResolvePlayTicketRequest) (*libraryv1.ResolvePlayTicketResponse, error) {
	actor := strings.TrimSpace(req.GetActorUserId())
	if actor == "" {
		return nil, status.Errorf(codes.InvalidArgument, "actor_user_id is required")
	}
	ticket := strings.TrimSpace(req.GetTicket())
	if ticket == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ticket is required")
	}
	pt, err := l.session.tickets.resolve(actor, ticket)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "stream link expired or is invalid")
	}
	return &libraryv1.ResolvePlayTicketResponse{
		TitleId:       pt.titleID,
		SourceId:      pt.sourceID,
		StreamUrl:     pt.streamURL,
		Authorization: pt.authorization,
	}, nil
}

// resolveCredsFromOneof implements the shared "credentials: supplied or
// session-cached" rule for calls taking a oneof {password, api_key}.
func (l *LibraryService) resolveCredsFromOneof(actor string, password *libraryv1.PasswordCredentials, apiKey string) (jellyfin.Credentials, error) {
	hasPassword := password != nil
	hasAPIKey := strings.TrimSpace(apiKey) != ""
	switch {
	case hasPassword && hasAPIKey:
		return jellyfin.Credentials{}, status.Errorf(codes.InvalidArgument, "provide either username/password or an API key, not both")
	case hasPassword:
		username := strings.TrimSpace(password.GetUsername())
		if username == "" || password.GetPassword() == "" {
			return jellyfin.Credentials{}, status.Errorf(codes.InvalidArgument, "username and password are required together")
		}
		return jellyfin.Credentials{Password: &jellyfin.PasswordCredentials{Username: username, Password: password.GetPassword()}}, nil
	case hasAPIKey:
		return jellyfin.Credentials{APIKey: strings.TrimSpace(apiKey)}, nil
	default:
		cached, ok := l.session.creds.Get(actor)
		if !ok {
			return jellyfin.Credentials{}, status.Errorf(codes.FailedPrecondition, "source credentials are required — enter them once for this session")
		}
		return cached, nil
	}
}
