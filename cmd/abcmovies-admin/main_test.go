package main

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
	"github.com/Nem-git/ABCMovies/internal/core"
	"github.com/Nem-git/ABCMovies/internal/store"
)

func runAdmin(args, stdin string) (string, error) {
	var stderr strings.Builder
	br := bufio.NewReader(strings.NewReader(stdin))
	err := run(context.Background(), strings.Fields(args), br, &stderr)
	return stderr.String(), err
}

func TestResetAdminRecovery(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "test.db")
	t.Setenv("ABCMOVIES_DB", db)
	t.Setenv("ABCMOVIES_AUDIT_LOG", filepath.Join(dir, "audit.log"))

	ctx := context.Background()

	// Uninitialized instance: recovery fails and is audited, without changes.
	_, err := runAdmin("reset-admin", "")
	if err == nil {
		t.Fatal("expected failure on uninitialized instance")
	}

	// Set up the initial admin and a session.
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	svc := core.NewIdentityService(st)
	setup, err := svc.SetupAdmin(ctx, &identityv1.SetupAdminRequest{Username: "admin", Password: "correct horse battery staple"})
	if err != nil {
		t.Fatal(err)
	}
	if setup.GetUserId() == "" || setup.GetSessionId() == "" {
		t.Fatal("setup returned no user_id/session_id")
	}

	// Aborts when the confirmation does not match.
	if _, err := runAdmin("reset-admin", "NOPE\n"); err == nil {
		t.Fatal("expected abort on mismatched confirmation")
	}

	// Reset with confirmation and new password.
	out, err := runAdmin("reset-admin", "RESET admin\nnew-test-pass-123\n")
	if err != nil {
		t.Fatalf("reset failed: %v (out=%q)", err, out)
	}
	if !strings.Contains(out, "1 session(s)") {
		t.Fatalf("expected session invalidated count in output, got %q", out)
	}

	// Old session is invalidated.
	v, err := svc.ValidateSession(ctx, &identityv1.ValidateSessionRequest{SessionId: setup.GetSessionId()})
	if err != nil {
		t.Fatal(err)
	}
	if v.GetValid() {
		t.Fatal("old session still valid after recovery")
	}

	// Old password no longer works; new one does.
	if _, err := svc.Login(ctx, &identityv1.LoginRequest{Username: "admin", Password: "correct horse battery staple"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("old password login err code = %v, want Unauthenticated", status.Code(err))
	}
	if _, err := svc.Login(ctx, &identityv1.LoginRequest{Username: "admin", Password: "new-test-pass-123"}); err != nil {
		t.Fatalf("new password login failed: %v", err)
	}

	// Audit log contains the recovery event and no secrets/hashes.
	data, err := os.ReadFile(filepath.Join(dir, "audit.log"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	for _, secret := range []string{"new-test-pass-123", "correct horse battery staple", "$argon2id"} {
		if strings.Contains(log, secret) {
			t.Fatalf("audit log contains secret/hash material: %q", secret)
		}
	}
	if !strings.Contains(log, `"event":"admin_recovery"`) {
		t.Fatalf("audit log missing admin_recovery event: %q", log)
	}
}

func TestResetAdminUsage(t *testing.T) {
	if _, err := runAdmin("usage-here", ""); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestReadLineEOF(t *testing.T) {
	_, err := readLine(bufio.NewReader(strings.NewReader("RESET admin")), "prompt")
	if err != nil {
		t.Fatalf("readLine without trailing newline returned %v", err)
	}
}
