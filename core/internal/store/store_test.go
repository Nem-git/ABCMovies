package store

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
)

// RunStoreSuite exercises every Store implementation against a common set of
// behavioral requirements. Each implementation's test file calls this with a
// fresh, empty store.
func RunStoreSuite(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()

	t.Run("GetMissing", func(t *testing.T) {
		s := newStore(t)
		_, err := s.Get(context.Background(), "no-such-key")
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("PutGetRoundTrip", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		val := []byte("hello, store")
		if err := s.Put(ctx, "k1", val); err != nil {
			t.Fatalf("put: %v", err)
		}
		got, err := s.Get(ctx, "k1")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if string(got) != string(val) {
			t.Fatalf("round-trip mismatch: got %q, want %q", got, val)
		}
	})

	t.Run("PutOverwrites", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		if err := s.Put(ctx, "k", []byte("v1")); err != nil {
			t.Fatalf("put v1: %v", err)
		}
		if err := s.Put(ctx, "k", []byte("v2")); err != nil {
			t.Fatalf("put v2: %v", err)
		}
		got, err := s.Get(ctx, "k")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if string(got) != "v2" {
			t.Fatalf("overwrite failed: got %q, want %q", got, "v2")
		}
	})

	t.Run("DeleteRemoves", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		if err := s.Put(ctx, "k", []byte("v")); err != nil {
			t.Fatalf("put: %v", err)
		}
		if err := s.Delete(ctx, "k"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		_, err := s.Get(ctx, "k")
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected ErrKeyNotFound after delete, got %v", err)
		}
	})

	t.Run("DeleteMissingIsNoop", func(t *testing.T) {
		s := newStore(t)
		if err := s.Delete(context.Background(), "no-such-key"); err != nil {
			t.Fatalf("delete of missing key should be noop, got %v", err)
		}
	})

	t.Run("ListEmpty", func(t *testing.T) {
		s := newStore(t)
		keys, err := s.List(context.Background(), "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(keys) != 0 {
			t.Fatalf("expected empty list, got %v", keys)
		}
	})

	t.Run("ListPrefix", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		for _, k := range []string{"user:1:a", "user:1:b", "user:2:a", "other:1"} {
			if err := s.Put(ctx, k, []byte("v")); err != nil {
				t.Fatalf("put %q: %v", k, err)
			}
		}
		keys, err := s.List(ctx, "user:1:")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		sort.Strings(keys)
		want := []string{"user:1:a", "user:1:b"}
		if len(keys) != len(want) {
			t.Fatalf("list prefix: got %v, want %v", keys, want)
		}
		for i := range keys {
			if keys[i] != want[i] {
				t.Fatalf("list prefix: got %v, want %v", keys, want)
			}
		}
	})

	t.Run("ListAll", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		for _, k := range []string{"a", "b", "c"} {
			if err := s.Put(ctx, k, []byte("v")); err != nil {
				t.Fatalf("put %q: %v", k, err)
			}
		}
		keys, err := s.List(ctx, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(keys) != 3 {
			t.Fatalf("list all: got %d keys, want 3", len(keys))
		}
	})

	t.Run("ValueIsolation", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		original := []byte("original")
		if err := s.Put(ctx, "k", original); err != nil {
			t.Fatalf("put: %v", err)
		}
		// Mutate the original slice — the store should not be affected.
		original[0] = 'X'
		got, err := s.Get(ctx, "k")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if string(got) != "original" {
			t.Fatalf("store was affected by external mutation: got %q", got)
		}
	})

	t.Run("ReturnValueIsolation", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		if err := s.Put(ctx, "k", []byte("v")); err != nil {
			t.Fatalf("put: %v", err)
		}
		got, _ := s.Get(ctx, "k")
		got[0] = 'X'
		got2, _ := s.Get(ctx, "k")
		if string(got2) != "v" {
			t.Fatalf("returned value was affected by external mutation: got %q", got2)
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		var wg sync.WaitGroup
		for i := range 50 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				key := "key"
				val := []byte{byte(i)}
				_ = s.Put(ctx, key, val)
				_, _ = s.Get(ctx, key)
				_, _ = s.List(ctx, "")
			}(i)
		}
		wg.Wait()
	})

	t.Run("EmptyValue", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		if err := s.Put(ctx, "k", []byte{}); err != nil {
			t.Fatalf("put empty: %v", err)
		}
		got, err := s.Get(ctx, "k")
		if err != nil {
			t.Fatalf("get empty: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty value, got %d bytes", len(got))
		}
	})

	t.Run("LargeValue", func(t *testing.T) {
		s := newStore(t)
		ctx := context.Background()
		val := make([]byte, 1024*1024) // 1 MiB
		for i := range val {
			val[i] = byte(i % 256)
		}
		if err := s.Put(ctx, "big", val); err != nil {
			t.Fatalf("put large: %v", err)
		}
		got, err := s.Get(ctx, "big")
		if err != nil {
			t.Fatalf("get large: %v", err)
		}
		if len(got) != len(val) {
			t.Fatalf("large value length: got %d, want %d", len(got), len(val))
		}
	})
}
