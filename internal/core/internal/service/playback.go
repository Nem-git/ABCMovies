package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

// Session credential cache and playback tickets.
//
// Both live in process memory only. Nothing here is ever written to the
// store, logged, or returned to browsers. This is the single place that
// changes when DEC-003 (Build 2) decides encrypted at-rest credential
// custody: the cache's storage moves behind this seam, the rest of the code
// does not.
const (
	// sessionCredTTL bounds how long source credentials linger in memory
	// without a successful use. Sliding: refreshed on every read.
	sessionCredTTL = 12 * time.Hour
	// ticketTTL bounds how long a minted playback ticket stays valid.
	// The video element re-requests (including Range) during playback; the
	// ticket must outlive the session's viewing but nothing longer.
	ticketTTL = 10 * time.Minute
)

type cachedCreds struct {
	creds     jellyfin.Credentials
	expiresAt time.Time
}

type sessionCredStore struct {
	mu sync.Mutex
	m  map[string]cachedCreds
}

func newSessionCredStore() *sessionCredStore { return &sessionCredStore{m: map[string]cachedCreds{}} }

func (s *sessionCredStore) Get(userID string) (jellyfin.Credentials, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.m[userID]
	if !ok {
		return jellyfin.Credentials{}, false
	}
	now := time.Now()
	if now.After(c.expiresAt) {
		delete(s.m, userID)
		return jellyfin.Credentials{}, false
	}
	c.expiresAt = now.Add(sessionCredTTL) // sliding expiry
	s.m[userID] = c
	return c.creds, true
}

func (s *sessionCredStore) Put(userID string, creds jellyfin.Credentials) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[userID] = cachedCreds{creds: creds, expiresAt: time.Now().Add(sessionCredTTL)}
}

type playTicket struct {
	actorUserID   string
	titleID       string
	sourceID      string
	streamURL     string
	authorization string
	expiresAt     time.Time
}

type ticketStore struct {
	mu sync.Mutex
	m  map[string]playTicket
}

func newTicketStore() *ticketStore { return &ticketStore{m: map[string]playTicket{}} }

var errTicketNotFound = errors.New("ticket not found or expired")

func (s *ticketStore) mint(t playTicket) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(time.Now())
	s.m[id] = t
	return id, nil
}

func (s *ticketStore) resolve(actorUserID, ticket string) (playTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(time.Now())
	t, ok := s.m[ticket]
	if !ok || t.actorUserID != actorUserID {
		// Uniform error: do not reveal whether the ticket exists.
		return playTicket{}, errTicketNotFound
	}
	return t, nil
}

func (s *ticketStore) purgeLocked(now time.Time) {
	for k, v := range s.m {
		if now.After(v.expiresAt) {
			delete(s.m, k)
		}
	}
}

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
	l.creds.Put(actor, creds)

	streamURL, urlErr := jellyfin.StreamURL(src.Endpoint, t.ExternalID)
	if urlErr != nil {
		return nil, status.Errorf(codes.Internal, "build stream URL: %v", urlErr.Message)
	}

	expiresAt := time.Now().Add(ticketTTL)
	id, err := l.tickets.mint(playTicket{
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
	pt, err := l.tickets.resolve(actor, ticket)
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
		cached, ok := l.creds.Get(actor)
		if !ok {
			return jellyfin.Credentials{}, status.Errorf(codes.FailedPrecondition, "source credentials are required — enter them once for this session")
		}
		return cached, nil
	}
}
