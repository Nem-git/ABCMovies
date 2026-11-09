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

func TestDeleteExpiredSessions(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	user, err := s.CreateUser(ctx, "admin", "test-hash-sentinel-not-a-real-digest")
	if err != nil {
		t.Fatal(err)
	}

	valid, err := s.CreateSession(ctx, user.UserID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := s.CreateSession(ctx, user.UserID, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	n, err := s.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("DeleteExpiredSessions removed %d, want 1", n)
	}

	if _, err := s.ValidateSession(ctx, valid.ID); err != nil {
		t.Fatalf("valid session missing after cleanup: %v", err)
	}
	// Expired session was never valid via ValidateSession; confirm it is gone by
	// ensuring cleanup is idempotent.
	n, err = s.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("second cleanup removed %d, want 0", n)
	}
	_ = expired
}

func TestSourceLifecycle(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	const active int32 = 1 // aligned to abcmovies.source.v1.SourceStatus_ACTIVE; mapped in core

	src, err := s.CreateSource(ctx, Source{
		ConnectorID:        "jellyfin",
		VerifiedExternalID: "srv-verified-id-1", // upstream identity, not a secret
		Endpoint:           "https://media.example.test",
		DisplayName:        "Living Room Jellyfin",
		Status:             active,
		CreatedBy:          "usr_actor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if src.SourceID == "" || src.CreatedBy != "usr_actor" {
		t.Fatalf("created source = %+v", src)
	}

	byID, err := s.GetSource(ctx, src.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if byID.VerifiedExternalID != "srv-verified-id-1" || byID.Status != active {
		t.Fatalf("GetSource = %+v", byID)
	}

	byVer, err := s.GetSourceByVerifiedIdentity(ctx, "jellyfin", "srv-verified-id-1")
	if err != nil {
		t.Fatal(err)
	}
	if byVer.SourceID != src.SourceID {
		t.Fatalf("dedup lookup source_id = %q, want %q", byVer.SourceID, src.SourceID)
	}

	list, err := s.ListSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].SourceID != src.SourceID {
		t.Fatalf("ListSources = %+v, want 1 source", list)
	}

	updated, err := s.UpdateSource(ctx, src.SourceID, "https://media2.example.test", "Renamed", active, "")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Endpoint != "https://media2.example.test" || updated.DisplayName != "Renamed" {
		t.Fatalf("UpdateSource = %+v", updated)
	}
	// Verified identity is immutable across updates.
	if updated.VerifiedExternalID != "srv-verified-id-1" {
		t.Fatalf("verified_external_id changed: %+v", updated)
	}
}

func TestSourceDuplicateVerifiedIdentity(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	const active int32 = 1
	_, err := s.CreateSource(ctx, Source{
		ConnectorID: "jellyfin", VerifiedExternalID: "srv-same",
		Endpoint: "https://a.example.test", DisplayName: "A", Status: active, CreatedBy: "usr_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Core dedups first; the UNIQUE constraint is the final guard.
	if _, err := s.CreateSource(ctx, Source{
		ConnectorID: "jellyfin", VerifiedExternalID: "srv-same",
		Endpoint: "https://b.example.test", DisplayName: "B", Status: active, CreatedBy: "usr_1",
	}); err == nil {
		t.Fatal("duplicate (connector_id, verified_external_id) unexpectedly succeeded")
	}
	// A different verified server is allowed.
	if _, err := s.CreateSource(ctx, Source{
		ConnectorID: "jellyfin", VerifiedExternalID: "srv-other",
		Endpoint: "https://c.example.test", DisplayName: "C", Status: active, CreatedBy: "usr_1",
	}); err != nil {
		t.Fatalf("different verified id: %v", err)
	}
}

func TestSourceNotFound(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	const errStatus int32 = 3 // aligned to abcmovies.source.v1.SourceStatus_ERROR
	if _, err := s.GetSource(ctx, "src_missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSource err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetSourceByVerifiedIdentity(ctx, "jellyfin", "srv_missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSourceByVerifiedIdentity err = %v, want ErrNotFound", err)
	}
	if _, err := s.UpdateSource(ctx, "src_missing", "https://x", "x", errStatus, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateSource err = %v, want ErrNotFound", err)
	}
}
