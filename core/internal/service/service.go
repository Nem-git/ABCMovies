// Package service implements the identity gRPC service.
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Nem-git/ABCMovies/core/internal/auth"
	"github.com/Nem-git/ABCMovies/core/internal/store"
	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
)

// sessionTTL is how long a login session lasts.
const sessionTTL = 7 * 24 * time.Hour

// minUsernameLength and minPasswordLength enforce basic hygiene on credentials.
// maxPasswordLength bounds argon2 work on attacker-controlled input.
const (
	minUsernameLength = 2
	minPasswordLength = 8
	maxPasswordLength = 1024
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
	initialized, err := i.store.HasUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read state: %v", err)
	}
	return &identityv1.GetStatusResponse{Initialized: initialized}, nil
}

// SetupAdmin creates the initial admin on an uninitialized instance and
// establishes a session for it (first-run setup behaves like registration).
func (i *IdentityService) SetupAdmin(ctx context.Context, req *identityv1.SetupAdminRequest) (*identityv1.SetupAdminResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	if len(username) < minUsernameLength {
		return nil, status.Errorf(codes.InvalidArgument, "username must be at least %d characters", minUsernameLength)
	}
	if len(req.GetPassword()) < minPasswordLength {
		return nil, status.Errorf(codes.InvalidArgument, "password must be at least %d characters", minPasswordLength)
	}
	if len(req.GetPassword()) > maxPasswordLength {
		return nil, status.Errorf(codes.InvalidArgument, "password must be at most %d characters", maxPasswordLength)
	}

	hash, err := auth.HashPassword(req.GetPassword())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash password: %v", err)
	}
	user, err := i.store.CreateInitialUser(ctx, username, hash)
	if err != nil {
		if errors.Is(err, store.ErrAlreadyInitialized) {
			return nil, status.Errorf(codes.FailedPrecondition, "instance already initialized")
		}
		return nil, status.Errorf(codes.Internal, "create user: %v", err)
	}
	sess, err := i.store.CreateSession(ctx, user.UserID, sessionTTL)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create session: %v", err)
	}
	return &identityv1.SetupAdminResponse{SessionId: sess.ID, UserId: user.UserID}, nil
}

// GetUser returns profile data for the user identified by user_id.
func (i *IdentityService) GetUser(ctx context.Context, req *identityv1.GetUserRequest) (*identityv1.GetUserResponse, error) {
	if strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id is required")
	}
	user, err := i.store.GetUserByID(ctx, req.GetUserId())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "read user: %v", err)
	}
	return &identityv1.GetUserResponse{UserId: user.UserID, Username: user.Username}, nil
}

// Login verifies credentials and creates a server-side session.
func (i *IdentityService) Login(ctx context.Context, req *identityv1.LoginRequest) (*identityv1.LoginResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	if username == "" || req.GetPassword() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username and password are required")
	}
	if len(req.GetPassword()) > maxPasswordLength {
		return nil, status.Errorf(codes.InvalidArgument, "username and password are required")
	}

	user, err := i.store.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Report as invalid credentials (do not reveal whether the username exists).
			return nil, status.Errorf(codes.Unauthenticated, "invalid username or password")
		}
		return nil, status.Errorf(codes.Internal, "look up user: %v", err)
	}
	match, err := auth.VerifyPassword(user.Hash, req.GetPassword())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify password: %v", err)
	}
	if !match {
		return nil, status.Errorf(codes.Unauthenticated, "invalid username or password")
	}

	sess, err := i.store.CreateSession(ctx, user.UserID, sessionTTL)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create session: %v", err)
	}
	return &identityv1.LoginResponse{SessionId: sess.ID, UserId: user.UserID}, nil
}

// ValidateSession checks whether a session is valid.
func (i *IdentityService) ValidateSession(ctx context.Context, req *identityv1.ValidateSessionRequest) (*identityv1.ValidateSessionResponse, error) {
	sess, err := i.store.ValidateSession(ctx, req.GetSessionId())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return &identityv1.ValidateSessionResponse{Valid: false}, nil
		}
		return nil, status.Errorf(codes.Internal, "validate session: %v", err)
	}
	return &identityv1.ValidateSessionResponse{Valid: true, UserId: sess.UserID}, nil
}

// Logout ends a session.
func (i *IdentityService) Logout(ctx context.Context, req *identityv1.LogoutRequest) (*identityv1.LogoutResponse, error) {
	if err := i.store.DeleteSession(ctx, req.GetSessionId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete session: %v", err)
	}
	return &identityv1.LogoutResponse{}, nil
}
