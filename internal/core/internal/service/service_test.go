package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

func newTestService(t *testing.T) (*IdentityService, *store.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewIdentityService(st), st
}

func setupAdmin(t *testing.T, svc *IdentityService) *identityv1.SetupAdminResponse {
	t.Helper()
	resp, err := svc.SetupAdmin(context.Background(), &identityv1.SetupAdminRequest{
		Username: "admin",
		Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatalf("SetupAdmin: %v", err)
	}
	if resp.GetUserId() == "" {
		t.Fatal("SetupAdmin returned no user_id")
	}
	if resp.GetSessionId() == "" {
		t.Fatal("SetupAdmin returned no session_id")
	}
	return resp
}

func TestSetupAdminEstablishesSession(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	resp := setupAdmin(t, svc)
	sess, err := svc.ValidateSession(ctx, &identityv1.ValidateSessionRequest{SessionId: resp.GetSessionId()})
	if err != nil {
		t.Fatal(err)
	}
	if !sess.GetValid() {
		t.Fatal("ValidateSession.Valid = false for the session returned by setup")
	}
	if sess.GetUserId() != resp.GetUserId() {
		t.Fatalf("session user_id = %q, want %q", sess.GetUserId(), resp.GetUserId())
	}
}

func TestGetStatusFreshAndInitialized(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	st, err := svc.GetStatus(ctx, &identityv1.GetStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if st.GetInitialized() {
		t.Fatal("GetStatus.Initialized = true on fresh instance")
	}

	setupAdmin(t, svc)
	st, err = svc.GetStatus(ctx, &identityv1.GetStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !st.GetInitialized() {
		t.Fatal("GetStatus.Initialized = false after setup")
	}
}

func TestSetupAdminFailsWhenInitialized(t *testing.T) {
	svc, _ := newTestService(t)
	setupAdmin(t, svc)

	_, err := svc.SetupAdmin(context.Background(), &identityv1.SetupAdminRequest{
		Username: "admin2",
		Password: "correct horse battery staple",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("SetupAdmin err code = %v, want FailedPrecondition", status.Code(err))
	}
}

func TestSetupAdminValidatesInput(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	if _, err := svc.SetupAdmin(ctx, &identityv1.SetupAdminRequest{Username: "a", Password: "correct horse battery staple"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("short username code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := svc.SetupAdmin(ctx, &identityv1.SetupAdminRequest{Username: "admin", Password: "short"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("short password code = %v, want InvalidArgument", status.Code(err))
	}
	if _, err := svc.SetupAdmin(ctx, &identityv1.SetupAdminRequest{Username: "admin", Password: strings.Repeat("x", maxPasswordLength+1)}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("long password code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestLoginRejectsOverlongPassword(t *testing.T) {
	svc, _ := newTestService(t)
	setupAdmin(t, svc)

	_, err := svc.Login(context.Background(), &identityv1.LoginRequest{
		Username: "admin",
		Password: strings.Repeat("x", maxPasswordLength+1),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Login overlong password code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestLoginValidateLogoutAndGetUser(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	created := setupAdmin(t, svc)

	login, err := svc.Login(ctx, &identityv1.LoginRequest{
		Username: "admin",
		Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatal(err)
	}
	if login.GetUserId() != created.GetUserId() {
		t.Fatalf("Login user_id = %q, want %q", login.GetUserId(), created.GetUserId())
	}
	if login.GetSessionId() == "" {
		t.Fatal("Login returned no session id")
	}

	sess, err := svc.ValidateSession(ctx, &identityv1.ValidateSessionRequest{SessionId: login.GetSessionId()})
	if err != nil {
		t.Fatal(err)
	}
	if !sess.GetValid() {
		t.Fatal("ValidateSession.Valid = false for fresh session")
	}
	if sess.GetUserId() != created.GetUserId() {
		t.Fatalf("ValidateSession.user_id = %q, want %q", sess.GetUserId(), created.GetUserId())
	}

	profile, err := svc.GetUser(ctx, &identityv1.GetUserRequest{UserId: created.GetUserId()})
	if err != nil {
		t.Fatal(err)
	}
	if profile.GetUsername() != "admin" {
		t.Fatalf("GetUser.username = %q, want admin", profile.GetUsername())
	}

	if _, err := svc.Logout(ctx, &identityv1.LogoutRequest{SessionId: login.GetSessionId()}); err != nil {
		t.Fatal(err)
	}
	sess, err = svc.ValidateSession(ctx, &identityv1.ValidateSessionRequest{SessionId: login.GetSessionId()})
	if err != nil {
		t.Fatal(err)
	}
	if sess.GetValid() {
		t.Fatal("ValidateSession.Valid = true after logout")
	}
}

func TestLoginRejectsWrongCredentialsWithoutProbing(t *testing.T) {
	svc, _ := newTestService(t)
	setupAdmin(t, svc)
	ctx := context.Background()

	wrongPass, err := svc.Login(ctx, &identityv1.LoginRequest{Username: "admin", Password: "wrong password"})
	unknownUser, err2 := svc.Login(ctx, &identityv1.LoginRequest{Username: "nobody", Password: "correct horse battery staple"})
	if wrongPass != nil || unknownUser != nil {
		t.Fatal("expected no login response for invalid credentials")
	}
	if status.Code(err) != codes.Unauthenticated || status.Code(err2) != codes.Unauthenticated {
		t.Fatalf("codes = (%v, %v), want Unauthenticated for both", status.Code(err), status.Code(err2))
	}
	if status.Convert(err).Message() != status.Convert(err2).Message() {
		t.Fatal("error messages reveal whether the username exists")
	}
}

func TestGetUserNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.GetUser(context.Background(), &identityv1.GetUserRequest{UserId: "usr_does_not_exist"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetUser code = %v, want NotFound", status.Code(err))
	}
}

func TestLoginAllowsMultipleSessions(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	setupAdmin(t, svc)

	l1, err := svc.Login(ctx, &identityv1.LoginRequest{Username: "admin", Password: "correct horse battery staple"})
	if err != nil {
		t.Fatal(err)
	}
	l2, err := svc.Login(ctx, &identityv1.LoginRequest{Username: "admin", Password: "correct horse battery staple"})
	if err != nil {
		t.Fatal(err)
	}
	if l1.GetSessionId() == l2.GetSessionId() {
		t.Fatal("re-login reused the session id")
	}
	for i, sess := range []string{l1.GetSessionId(), l2.GetSessionId()} {
		v, err := svc.ValidateSession(ctx, &identityv1.ValidateSessionRequest{SessionId: sess})
		if err != nil {
			t.Fatal(err)
		}
		if !v.GetValid() {
			t.Fatalf("session %d is not valid; multiple sessions must remain valid", i+1)
		}
		if v.GetUserId() != l1.GetUserId() {
			t.Fatalf("session %d user_id = %q, want %q", i+1, v.GetUserId(), l1.GetUserId())
		}
	}
}
