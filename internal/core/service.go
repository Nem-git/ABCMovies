// Package core implements the identity gRPC service.
package core

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Nem-git/ABCMovies/internal/auth"
	"github.com/Nem-git/ABCMovies/internal/store"
	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
)

// sessionTTL is how long a login session lasts.
const sessionTTL = 7 * 24 * time.Hour

// minUsernameLength and minPasswordLength enforce basic hygiene on credentials.
const (
	minUsernameLength = 2
	minPasswordLength = 8
)

// IdentityService implements identityv1.IdentityServiceServer.
type IdentityService struct {
	identityv1.UnimplementedIdentityServiceServer

	store *store.Store
}

// NewIdentityService returns the identity service backed by store.
func NewIdentityService(s *store.Store) *IdentityService {
	return &IdentityService{store: s}
}

// GetStatus reports whether the instance has been initialized.
func (i *IdentityService) GetStatus(ctx context.Context, _ *identityv1.GetStatusRequest) (*identityv1.GetStatusResponse, error) {
	n, err := i.store.AdminCount(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read state: %v", err)
	}
	return &identityv1.GetStatusResponse{Initialized: n > 0}, nil
}

// SetupAdmin creates the initial admin on an uninitialized instance.
func (i *IdentityService) SetupAdmin(ctx context.Context, req *identityv1.SetupAdminRequest) (*identityv1.SetupAdminResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	if len(username) < minUsernameLength {
		return nil, status.Errorf(codes.InvalidArgument, "username must be at least %d characters", minUsernameLength)
	}
	if len(req.GetPassword()) < minPasswordLength {
		return nil, status.Errorf(codes.InvalidArgument, "password must be at least %d characters", minPasswordLength)
	}

	n, err := i.store.AdminCount(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read state: %v", err)
	}
	if n > 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "instance already initialized")
	}

	hash, err := auth.HashPassword(req.GetPassword())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash password: %v", err)
	}
	if err := i.store.CreateAdmin(ctx, username, hash); err != nil {
		return nil, status.Errorf(codes.Internal, "create admin: %v", err)
	}
	return &identityv1.SetupAdminResponse{}, nil
}

// Login verifies credentials and creates a server-side session.
func (i *IdentityService) Login(ctx context.Context, req *identityv1.LoginRequest) (*identityv1.LoginResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	if username == "" || req.GetPassword() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username and password are required")
	}

	admin, err := i.store.GetAdmin(ctx, username)
	if err != nil {
		// Report as invalid credentials (do not reveal whether the username exists).
		return nil, status.Errorf(codes.Unauthenticated, "invalid username or password")
	}
	match, err := auth.VerifyPassword(admin.Hash, req.GetPassword())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify password: %v", err)
	}
	if !match {
		return nil, status.Errorf(codes.Unauthenticated, "invalid username or password")
	}

	sess, err := i.store.CreateSession(ctx, username, sessionTTL)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create session: %v", err)
	}
	return &identityv1.LoginResponse{SessionId: sess.ID, Username: username}, nil
}

// ValidateSession checks whether a session is valid.
func (i *IdentityService) ValidateSession(ctx context.Context, req *identityv1.ValidateSessionRequest) (*identityv1.ValidateSessionResponse, error) {
	sess, err := i.store.ValidateSession(ctx, req.GetSessionId())
	if err != nil {
		return &identityv1.ValidateSessionResponse{Valid: false}, nil
	}
	return &identityv1.ValidateSessionResponse{Valid: true, Username: sess.Username}, nil
}

// Logout ends a session.
func (i *IdentityService) Logout(ctx context.Context, req *identityv1.LogoutRequest) (*identityv1.LogoutResponse, error) {
	if err := i.store.DeleteSession(ctx, req.GetSessionId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete session: %v", err)
	}
	return &identityv1.LogoutResponse{}, nil
}