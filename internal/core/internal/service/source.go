package service

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sourcev1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/source/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

// maxDisplayNameLength bounds user-supplied display names; values in this
// range are shown in the UI and stored in the database.
const maxDisplayNameLength = 128

// sourceVerifier is the verification dependency for a source connector.
// Tests replace it with a fake so no real Jellyfin server is needed.
type sourceVerifier interface {
	Verify(ctx context.Context, endpoint string, creds jellyfin.Credentials) (*jellyfin.ServerInfo, *jellyfin.Error)
}

// sourceConnector is a source connector name and its verifier. Core owns
// connector selection and shared validation; connector implementations own
// verification semantics.
type sourceConnector struct {
	ID       string
	Verifier sourceVerifier
}

// SourceService implements sourcev1.SourceServiceServer.
type SourceService struct {
	sourcev1.UnimplementedSourceServiceServer

	store      *store.Store
	connectors map[string]sourceVerifier
	session    *SessionState
}

// NewSourceService returns a SourceService backed by the jellyfin connector.
// sv is the shared in-memory session state (core wires one instance for all
// services).
func NewSourceService(s *store.Store, sv *SessionState) *SourceService {
	return NewSourceServiceWithVerifier(s, jellyfin.NewClient(), sv)
}

// NewSourceServiceWithVerifier returns a SourceService with a chosen verifier
// for the "jellyfin" connector. Tests pass a fake here. sv is the shared
// in-memory session state.
func NewSourceServiceWithVerifier(s *store.Store, v sourceVerifier, sv *SessionState) *SourceService {
	return &SourceService{store: s, connectors: map[string]sourceVerifier{"jellyfin": v}, session: sv}
}

// registerConnector binds an id to a verifier. Supports custom connectors in
// tests and future service wiring.
func (s *SourceService) registerConnector(id string, v sourceVerifier) {
	if id == "" {
		return
	}
	s.connectors[id] = v
}

// ConnectSource verifies and connects a source. Credentials are input-only.
func (s *SourceService) ConnectSource(ctx context.Context, req *sourcev1.ConnectSourceRequest) (*sourcev1.ConnectSourceResponse, error) {
	actor := strings.TrimSpace(req.GetActorUserId())
	if actor == "" {
		return nil, status.Errorf(codes.InvalidArgument, "actor_user_id is required")
	}
	connectorID := strings.TrimSpace(req.GetConnectorId())
	if connectorID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "connector_id is required")
	}
	verifier, ok := s.connectors[connectorID]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported connector_id %q", connectorID)
	}
	endpointRaw := strings.TrimSpace(req.GetEndpoint())
	if endpointRaw == "" {
		return nil, status.Errorf(codes.InvalidArgument, "endpoint is required")
	}
	displayNameRaw := strings.TrimSpace(req.GetDisplayName())
	if len(displayNameRaw) > maxDisplayNameLength {
		return nil, status.Errorf(codes.InvalidArgument, "display_name must be at most %d characters", maxDisplayNameLength)
	}
	creds, err := credentialsFromSourceRequest(req)
	if err != nil {
		return nil, err
	}
	verified, vErr := verifier.Verify(ctx, endpointRaw, creds)
	if vErr != nil {
		return nil, mapVerifyError(vErr)
	}
	displayName := displayNameRaw
	if displayName == "" {
		displayName = safeDisplayName(verified.DisplayName, 128, endpointRaw)
	}

	// Duplicate detection (DEC-001): verified identity is authoritative, so
	// look up first to avoid creating a second row. The store's UNIQUE
	// constraint on (connector_id, verified_external_id) is the final guard
	// against races (e.g. two simultaneous connects) — if insertion fails as a
	// duplicate, fall back to returning the existing row.
	if existing, err := s.store.GetSourceByVerifiedIdentity(ctx, connectorID, verified.VerifiedID); err == nil {
		return duplicateConnectSourceResponse(existing), nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, status.Errorf(codes.Internal, "read sources: %v", err)
	}

	created, err := s.store.CreateSource(ctx, store.Source{
		ConnectorID:        connectorID,
		VerifiedExternalID: verified.VerifiedID,
		Endpoint:           verified.Endpoint,
		DisplayName:        displayName,
		Status:             int32(sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE),
		CreatedBy:          actor,
	})
	if errors.Is(err, store.ErrDuplicateVerifiedIdentity) {
		existing, lerr := s.store.GetSourceByVerifiedIdentity(ctx, connectorID, verified.VerifiedID)
		if lerr == nil {
			return duplicateConnectSourceResponse(existing), nil
		}
		return nil, status.Errorf(codes.Internal, "read source after duplicate insert: %v", lerr)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create source: %v", err)
	}
	return &sourcev1.ConnectSourceResponse{
		Outcome: sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_CONNECTED,
		Source:  sourceToProto(created),
	}, nil
}

// UpdateSource applies allowed changes. Endpoint/connection changes require
// fresh credentials, re-verification, and the same verified external identity
// (no silent reassociation).
func (s *SourceService) UpdateSource(ctx context.Context, req *sourcev1.UpdateSourceRequest) (*sourcev1.UpdateSourceResponse, error) {
	actor := strings.TrimSpace(req.GetActorUserId())
	if actor == "" {
		return nil, status.Errorf(codes.InvalidArgument, "actor_user_id is required")
	}
	sourceID := strings.TrimSpace(req.GetSourceId())
	if sourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "source_id is required")
	}
	existing, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "source not found")
		}
		return nil, status.Errorf(codes.Internal, "read source: %v", err)
	}
	verifier, ok := s.connectors[existing.ConnectorID]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "source connector unsupported")
	}
	displayNameRaw := strings.TrimSpace(req.GetDisplayName())
	if len(displayNameRaw) > maxDisplayNameLength {
		return nil, status.Errorf(codes.InvalidArgument, "display_name must be at most %d characters", maxDisplayNameLength)
	}

	endpointRaw := strings.TrimSpace(req.GetEndpoint())
	targetEndpoint := existing.Endpoint

	if endpointRaw != "" {
		normalized, nerr := jellyfin.NormalizeEndpoint(endpointRaw)
		if nerr != nil {
			return nil, mapVerifyError(nerr)
		}
		targetEndpoint = normalized
	}
	endpointChanged := targetEndpoint != existing.Endpoint
	reqCreds, cErr := credentialsFromUpdateRequest(req)
	if cErr != nil {
		return nil, cErr
	}

	statusForStore := existing.Status
	statusDetail := existing.StatusDetail
	if endpointChanged || reqCreds != nil {
		if reqCreds == nil {
			return nil, status.Errorf(codes.InvalidArgument, "fresh credentials are required to change the endpoint")
		}
		verified, vErr := verifier.Verify(ctx, targetEndpoint, *reqCreds)
		if vErr != nil {
			return nil, mapVerifyError(vErr)
		}
		if verified.VerifiedID != existing.VerifiedExternalID {
			return nil, status.Errorf(codes.FailedPrecondition, "endpoint resolves to a different upstream server; update cannot reassign this source to a different verified server")
		}
		targetEndpoint = verified.Endpoint
		statusForStore = int32(sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE)
		statusDetail = ""
	}

	displayName := existing.DisplayName
	if displayNameRaw != "" {
		displayName = safeDisplayName(displayNameRaw, maxDisplayNameLength, existing.DisplayName)
	}

	updated, err := s.store.UpdateSource(ctx, sourceID, targetEndpoint, displayName, statusForStore, statusDetail)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "source not found")
		}
		return nil, status.Errorf(codes.Internal, "update source: %v", err)
	}
	return &sourcev1.UpdateSourceResponse{Source: sourceToProto(updated)}, nil
}

// ListSources returns non-sensitive source summaries.
func (s *SourceService) ListSources(ctx context.Context, req *sourcev1.ListSourcesRequest) (*sourcev1.ListSourcesResponse, error) {
	actor := strings.TrimSpace(req.GetActorUserId())
	if actor == "" {
		return nil, status.Errorf(codes.InvalidArgument, "actor_user_id is required")
	}
	rows, err := s.store.ListSources(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list sources: %v", err)
	}
	out := make([]*sourcev1.Source, 0, len(rows))
	for _, row := range rows {
		out = append(out, sourceToProto(row))
	}
	return &sourcev1.ListSourcesResponse{Sources: out}, nil
}

// GetSource returns one non-sensitive source summary.
func (s *SourceService) GetSource(ctx context.Context, req *sourcev1.GetSourceRequest) (*sourcev1.GetSourceResponse, error) {
	actor := strings.TrimSpace(req.GetActorUserId())
	if actor == "" {
		return nil, status.Errorf(codes.InvalidArgument, "actor_user_id is required")
	}
	sourceID := strings.TrimSpace(req.GetSourceId())
	if sourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "source_id is required")
	}
	src, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "source not found")
		}
		return nil, status.Errorf(codes.Internal, "get source: %v", err)
	}
	return &sourcev1.GetSourceResponse{Source: sourceToProto(src)}, nil
}

func duplicateConnectSourceResponse(existing *store.Source) *sourcev1.ConnectSourceResponse {
	return &sourcev1.ConnectSourceResponse{
		Outcome: sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_DUPLICATE,
		Source:  sourceToProto(existing),
		Detail:  "this server is already connected",
	}
}

// credentialsFromSourceRequest builds connector credentials from a
// ConnectSourceRequest. Exactly one credential form must be provided.
func credentialsFromSourceRequest(req *sourcev1.ConnectSourceRequest) (jellyfin.Credentials, error) {
	hasPassword := req.GetPassword() != nil
	hasAPIKey := strings.TrimSpace(req.GetApiKey()) != ""
	var creds jellyfin.Credentials
	switch {
	case hasPassword && hasAPIKey:
		return jellyfin.Credentials{}, status.Errorf(codes.InvalidArgument, "provide either username/password or an API key, not both")
	case hasPassword:
		p := req.GetPassword()
		username := strings.TrimSpace(p.GetUsername())
		if username == "" || p.GetPassword() == "" {
			return jellyfin.Credentials{}, status.Errorf(codes.InvalidArgument, "username and password are required together")
		}
		creds.Password = &jellyfin.PasswordCredentials{Username: username, Password: p.GetPassword()}
	case hasAPIKey:
		creds.APIKey = strings.TrimSpace(req.GetApiKey())
	default:
		return jellyfin.Credentials{}, status.Errorf(codes.InvalidArgument, "credentials are required")
	}
	return creds, nil
}

// credentialsFromUpdateRequest builds fresh connector credentials from an
// UpdateSourceRequest. Returns (nil, nil) when no credentials were provided.
func credentialsFromUpdateRequest(req *sourcev1.UpdateSourceRequest) (*jellyfin.Credentials, error) {
	hasPassword := req.GetPassword() != nil
	hasAPIKey := strings.TrimSpace(req.GetApiKey()) != ""
	var creds jellyfin.Credentials
	switch {
	case hasPassword && hasAPIKey:
		return nil, status.Errorf(codes.InvalidArgument, "provide either username/password or an API key, not both")
	case hasPassword:
		p := req.GetPassword()
		username := strings.TrimSpace(p.GetUsername())
		if username == "" || p.GetPassword() == "" {
			return nil, status.Errorf(codes.InvalidArgument, "username and password are required together")
		}
		creds.Password = &jellyfin.PasswordCredentials{Username: username, Password: p.GetPassword()}
		return &creds, nil
	case hasAPIKey:
		creds.APIKey = strings.TrimSpace(req.GetApiKey())
		return &creds, nil
	default:
		return nil, nil
	}
}

// mapVerifyError maps a connector verification error to a gRPC status with a
// safe, actionable message. It never includes credentials or tokens.
func mapVerifyError(err *jellyfin.Error) error {
	if err == nil {
		return nil
	}
	switch err.Kind {
	case jellyfin.ErrKindInvalidEndpoint:
		return status.Errorf(codes.InvalidArgument, "%s", err.Message)
	case jellyfin.ErrKindUnreachable:
		return status.Errorf(codes.Unavailable, "%s", err.Message)
	case jellyfin.ErrKindUnauthorized:
		return status.Errorf(codes.Unauthenticated, "%s", err.Message)
	case jellyfin.ErrKindNotFound:
		return status.Errorf(codes.NotFound, "%s: check the server address and path", err.Message)
	case jellyfin.ErrKindMisconfig:
		return status.Errorf(codes.FailedPrecondition, "%s", err.Message)
	case jellyfin.ErrKindUpstreamError:
		return status.Errorf(codes.Unavailable, "%s", err.Message)
	default:
		return status.Errorf(codes.Internal, "%s", err.Message)
	}
}

func safeDisplayName(name string, max int, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		if strings.TrimSpace(fallback) == "" {
			return "Unknown server"
		}
		return fallback
	}
	if len(name) > max {
		return name[:max]
	}
	return name
}

func sourceStatusToProto(v int32) sourcev1.SourceStatus {
	switch v {
	case int32(sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE):
		return sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE
	case int32(sourcev1.SourceStatus_SOURCE_STATUS_NEEDS_REVERIFICATION):
		return sourcev1.SourceStatus_SOURCE_STATUS_NEEDS_REVERIFICATION
	case int32(sourcev1.SourceStatus_SOURCE_STATUS_ERROR):
		return sourcev1.SourceStatus_SOURCE_STATUS_ERROR
	default:
		return sourcev1.SourceStatus_SOURCE_STATUS_UNSPECIFIED
	}
}

func sourceToProto(src *store.Source) *sourcev1.Source {
	return &sourcev1.Source{
		SourceId:           src.SourceID,
		ConnectorId:        src.ConnectorID,
		VerifiedExternalId: src.VerifiedExternalID,
		Endpoint:           src.Endpoint,
		DisplayName:        src.DisplayName,
		Status:             sourceStatusToProto(src.Status),
		StatusDetail:       src.StatusDetail,
	}
}
