package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUserLifecycle(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	initialized, err := s.HasUser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if initialized {
		t.Fatal("HasUser = true, want false before setup")
	}

	// Stored hashes are opaque to the store; it persists whatever value it is
	// given. Hashing itself is tested in package auth. Use an unmistakably
	// fake sentinel, not something that resembles a real argon2 digest.
	const sentinelHash = "test-hash-sentinel-not-a-real-digest"
	user, err := s.CreateUser(ctx, "admin", sentinelHash)
	if err != nil {
		t.Fatal(err)
	}
	if user.UserID == "" {
		t.Fatal("UserID is empty, want generated id")
	}

	byName, err := s.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if byName.UserID != user.UserID {
		t.Fatalf("GetUserByUsername UserID = %q, want %q", byName.UserID, user.UserID)
	}
	if byName.Hash != sentinelHash {
		t.Fatalf("GetUserByUsername Hash = %q, want the stored sentinel %q", byName.Hash, sentinelHash)
	}

	byID, err := s.GetUserByID(ctx, user.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if byID.Username != "admin" {
		t.Fatalf("GetUserByID Username = %q, want admin", byID.Username)
	}
	if byID.Hash != sentinelHash {
		t.Fatalf("GetUserByID Hash = %q, want the stored sentinel %q", byID.Hash, sentinelHash)
	}

	initialized, err = s.HasUser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !initialized {
		t.Fatal("HasUser = false, want true after setup")
	}

	if _, err := s.CreateUser(ctx, "admin", sentinelHash); err == nil {
		t.Fatal("duplicate username unexpectedly succeeded")
	}
}

func TestGetUserNotFound(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if _, err := s.GetUserByUsername(ctx, "nobody"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUserByUsername err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetUserByID(ctx, "usr_missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUserByID err = %v, want ErrNotFound", err)
	}
}

func TestCreateInitialUser(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	user, err := s.CreateInitialUser(ctx, "admin", "test-hash-sentinel-not-a-real-digest")
	if err != nil {
		t.Fatalf("CreateInitialUser: %v", err)
	}
	if user.UserID == "" {
		t.Fatal("CreateInitialUser returned no user_id")
	}

	if _, err := s.CreateInitialUser(ctx, "other", "test-hash-sentinel-not-a-real-digest"); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second CreateInitialUser err = %v, want ErrAlreadyInitialized", err)
	}
}

func TestCreateInitialUserConcurrent(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	users := make([]*User, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			users[i], errs[i] = s.CreateInitialUser(ctx, "admin", "test-hash-sentinel-not-a-real-digest")
		}(i)
	}
	wg.Wait()

	var okCount int
	for i := 0; i < n; i++ {
		if errs[i] == nil {
			okCount++
			if users[i] == nil || users[i].UserID == "" {
				t.Fatalf("attempt %d succeeded without user", i)
			}
			continue
		}
		if !errors.Is(errs[i], ErrAlreadyInitialized) {
			t.Fatalf("attempt %d err = %v, want nil or ErrAlreadyInitialized", i, errs[i])
		}
	}
	if okCount != 1 {
		t.Fatalf("successful initializations = %d, want 1", okCount)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	user, err := s.CreateUser(ctx, "admin", "test-hash-sentinel-not-a-real-digest")
	if err != nil {
		t.Fatal(err)
	}

	sess, err := s.CreateSession(ctx, user.UserID, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserID != user.UserID {
		t.Fatalf("Session UserID = %q, want %q", sess.UserID, user.UserID)
	}
	got, err := s.ValidateSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != user.UserID {
		t.Fatalf("ValidateSession UserID = %q, want %q", got.UserID, user.UserID)
	}

	if err := s.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ValidateSession(ctx, sess.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ValidateSession after DeleteSession err = %v, want ErrNotFound", err)
	}
}
