// Command abcmovies-admin is the operator-side administration tool for an
// ABCMovies instance. It is not exposed over HTTP and requires local host
// access (run where the instance's database lives). It exists to support
// first-class local recovery per decisions/product/0001 (R1): it never
// prints credentials, hashes, or session material, it affects only the
// targeted local admin account, and it records an audit/event log entry.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nem-git/ABCMovies/internal/config"
	"github.com/Nem-git/ABCMovies/internal/core/internal/audit"
	"github.com/Nem-git/ABCMovies/internal/core/internal/auth"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

func auditPath(dbPath string) string {
	if p := os.Getenv("ABCMOVIES_AUDIT_LOG"); p != "" {
		return p
	}
	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		return "abcmovies-audit.log"
	}
	return filepath.Join(dir, "abcmovies-audit.log")
}

func readLine(r *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func run(ctx context.Context, args []string, stdin *bufio.Reader, stderr io.Writer) error {
	if len(args) != 1 || (args[0] != "reset-admin" && args[0] != "reset-admin-password") {
		return errors.New("usage: abcmovies-admin reset-admin (local operator recovery; see DEC-009/ADR 0001)")
	}

	dbPath := config.Env("ABCMOVIES_DB", "data/abcmovies.db")
	logPath := auditPath(dbPath)

	aud, err := audit.Open(logPath)
	if err != nil {
		return fmt.Errorf("open audit log %q: %w", logPath, err)
	}
	defer func() { _ = aud.Close() }()

	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database %q: %w", dbPath, err)
	}
	defer func() { _ = st.Close() }()

	admin, err := st.GetAdminCredentials(ctx)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			_ = aud.Log(audit.Event{Event: "admin_recovery_failed", Detail: "instance not initialized"})
			return errors.New("instance is not initialized; no admin account exists")
		}
		_ = aud.Log(audit.Event{Event: "admin_recovery_failed", Detail: err.Error()})
		return err
	}

	// Confirmation: operator must confirm against the actual target.
	confirm, err := readLine(stdin, fmt.Sprintf("Reset the password for the local admin %q (user_id %s)? Type RESET %s: ", admin.Username, admin.UserID, admin.Username))
	if err != nil {
		_ = aud.Log(audit.Event{Event: "admin_recovery_failed", UserID: admin.UserID, Username: admin.Username, Detail: "read confirmation"})
		return fmt.Errorf("read confirmation: %w", err)
	}
	if confirm != "RESET "+admin.Username {
		_ = aud.Log(audit.Event{Event: "admin_recovery_aborted", UserID: admin.UserID, Username: admin.Username})
		return errors.New("confirmation did not match; aborting without changes")
	}

	password, err := readLine(stdin, "New admin password (will be echoed; prefer piping): ")
	if err != nil {
		_ = aud.Log(audit.Event{Event: "admin_recovery_failed", UserID: admin.UserID, Username: admin.Username, Detail: "read password"})
		return fmt.Errorf("read password: %w", err)
	}
	if len(password) < 8 {
		_ = aud.Log(audit.Event{Event: "admin_recovery_failed", UserID: admin.UserID, Username: admin.Username, Detail: "new password too short"})
		return errors.New("password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		_ = aud.Log(audit.Event{Event: "admin_recovery_failed", UserID: admin.UserID, Username: admin.Username, Detail: "hash password"})
		return fmt.Errorf("hash password: %w", err)
	}

	sessions, err := st.SetPassword(ctx, admin.UserID, hash)
	if err != nil {
		_ = aud.Log(audit.Event{Event: "admin_recovery_failed", UserID: admin.UserID, Username: admin.Username, Detail: "update credential"})
		return fmt.Errorf("update admin credential: %w", err)
	}

	if err := aud.Log(audit.Event{Event: "admin_recovery", UserID: admin.UserID, Username: admin.Username, Detail: fmt.Sprintf("sessions invalidated: %d", sessions)}); err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	fmt.Fprintf(stderr, "admin password reset; %d session(s) invalidated; audit log: %s\n", sessions, logPath)
	return nil
}

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args[1:], bufio.NewReader(os.Stdin), os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
