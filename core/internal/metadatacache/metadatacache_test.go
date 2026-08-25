package metadatacache

import (
	"context"
	"path/filepath"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/protobuf/proto"
)

// backends returns one cache per storage backend the class supports; every
// behavioral test runs against all of them (storage-class discipline,
// PLAN.md §2.4).
func backends(t *testing.T) []*Cache {
	t.Helper()
	ctx := context.Background()
	inmem, err := New(store.NewInMemory(), nil)
	if err != nil {
		t.Fatalf("in-memory: %v", err)
	}
	path := filepath.Join(t.TempDir(), "metadata-cache.db")
	st, err := store.NewSQLite(ctx, path)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sqlite, err := New(st, nil)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	return []*Cache{inmem, sqlite}
}

func sampleRecord() *corev1.TitleMetadata {
	return &corev1.TitleMetadata{
		Title:            "The Matrix",
		Year:             1999,
		Description:      "Reality is a simulation.",
		Rating:           8.7,
		Genres:           []string{"Science Fiction", "Action"},
		Directors:        []string{"Lana Wachowski", "Lilly Wachowski"},
		OriginalLanguage: "en",
		KindSpecific:     &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{RuntimeMinutes: 136}},
		Source: map[string]*corev1.EnrichmentSource{
			"base.title":     {Slot: "catalogue:tmdb"},
			"base.rating":    {Slot: "catalogue:tmdb"},
			"base.directors": {Slot: "provider:home-jellyfin"},
		},
		Extra: map[string]string{"tmdb.tagline": "Free your mind"},
	}
}

func TestRecordRoundTripPreservesFieldsAndProvenance(t *testing.T) {
	ctx := context.Background()
	for _, c := range backends(t) {
		want := sampleRecord()
		if err := c.PutRecord(ctx, "tmdb:603", want); err != nil {
			t.Fatalf("PutRecord: %v", err)
		}
		got, ok, err := c.GetRecord(ctx, "tmdb:603")
		if err != nil || !ok {
			t.Fatalf("GetRecord = (%v, %v)", ok, err)
		}
		if !proto.Equal(want, got) {
			t.Fatalf("round trip lost fidelity:\n got %v\nwant %v", got, want)
		}
	}
}

func TestAliasResolvesAndCanonicalFallsBack(t *testing.T) {
	ctx := context.Background()
	for _, c := range backends(t) {
		if err := c.PutRecord(ctx, "tmdb:603", sampleRecord()); err != nil {
			t.Fatalf("PutRecord: %v", err)
		}
		if err := c.LinkAlias(ctx, "imdb:tt0133093", "tmdb:603"); err != nil {
			t.Fatalf("LinkAlias: %v", err)
		}
		ref, ok, err := c.Resolve(ctx, "imdb:tt0133093")
		if err != nil || !ok || ref != "tmdb:603" {
			t.Fatalf("alias Resolve = (%q, %v, %v)", ref, ok, err)
		}
		ref, ok, err = c.Resolve(ctx, "tmdb:603")
		if err != nil || !ok || ref != "tmdb:603" {
			t.Fatalf("canonical Resolve = (%q, %v, %v)", ref, ok, err)
		}
		if _, ok, _ := c.Resolve(ctx, "tmdb:999999"); ok {
			t.Fatal("unknown ID resolved")
		}
	}
}

func TestLinkAliasRequiresExistingRecord(t *testing.T) {
	ctx := context.Background()
	for _, c := range backends(t) {
		if err := c.LinkAlias(ctx, "imdb:tt0133093", "tmdb:603"); err == nil {
			t.Fatal("alias linked to a missing record")
		}
		if err := c.PutRecord(ctx, "tmdb:603", sampleRecord()); err != nil {
			t.Fatalf("PutRecord: %v", err)
		}
		if err := c.LinkAlias(ctx, "imdb:tt0133093", "tmdb:603"); err != nil {
			t.Fatalf("LinkAlias after record exists: %v", err)
		}
	}
}

func TestDeleteRecordPurgesAliasesToo(t *testing.T) {
	ctx := context.Background()
	for _, c := range backends(t) {
		if err := c.PutRecord(ctx, "tmdb:603", sampleRecord()); err != nil {
			t.Fatalf("PutRecord: %v", err)
		}
		if err := c.LinkAlias(ctx, "imdb:tt0133093", "tmdb:603"); err != nil {
			t.Fatalf("LinkAlias: %v", err)
		}
		if err := c.LinkAlias(ctx, "tmdb:old-603", "tmdb:603"); err != nil {
			t.Fatalf("LinkAlias second: %v", err)
		}
		// An unrelated alias must survive the purge.
		if err := c.PutRecord(ctx, "tmdb:604", sampleRecord()); err != nil {
			t.Fatalf("PutRecord other: %v", err)
		}
		if err := c.LinkAlias(ctx, "imdb:tt0134044", "tmdb:604"); err != nil {
			t.Fatalf("LinkAlias other: %v", err)
		}

		if err := c.DeleteRecord(ctx, "tmdb:603"); err != nil {
			t.Fatalf("DeleteRecord: %v", err)
		}
		if _, ok, _ := c.GetRecord(ctx, "tmdb:603"); ok {
			t.Fatal("record survived deletion")
		}
		if _, ok, _ := c.Resolve(ctx, "imdb:tt0133093"); ok {
			t.Fatal("alias survived record purge")
		}
		if _, ok, _ := c.Resolve(ctx, "tmdb:old-603"); ok {
			t.Fatal("second alias survived record purge")
		}
		if _, ok, _ := c.Resolve(ctx, "imdb:tt0134044"); !ok {
			t.Fatal("unrelated alias was purged")
		}
	}
}

func TestMalformedExternalIDsRejected(t *testing.T) {
	ctx := context.Background()
	c := backends(t)[0]
	for _, id := range []string{"", "no-separator", ":leading-empty", "trailing-empty:"} {
		if err := c.PutRecord(ctx, id, sampleRecord()); err == nil {
			t.Fatalf("PutRecord accepted %q", id)
		}
		if _, _, err := c.Resolve(ctx, id); err == nil {
			t.Fatalf("Resolve accepted %q", id)
		}
		if err := c.LinkAlias(ctx, id, "tmdb:1"); err == nil {
			t.Fatalf("LinkAlias accepted bad alias %q", id)
		}
	}
}

func TestSQLiteDurabilityAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "metadata-cache.db")

	open := func() *Cache {
		t.Helper()
		st, err := store.NewSQLite(ctx, path)
		if err != nil {
			t.Fatalf("NewSQLite: %v", err)
		}
		t.Cleanup(func() { _ = st.Close() })
		c, err := New(st, nil)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return c
	}

	first := open()
	if err := first.PutRecord(ctx, "tmdb:603", sampleRecord()); err != nil {
		t.Fatalf("PutRecord: %v", err)
	}
	if err := first.LinkAlias(ctx, "imdb:tt0133093", "tmdb:603"); err != nil {
		t.Fatalf("LinkAlias: %v", err)
	}

	reopened := open()
	got, ok, err := reopened.GetRecord(ctx, "tmdb:603")
	if err != nil || !ok || got.GetTitle() != "The Matrix" {
		t.Fatalf("after reopen: ok=%v err=%v title=%q", ok, err, got.GetTitle())
	}
	ref, ok, err := reopened.Resolve(ctx, "imdb:tt0133093")
	if err != nil || !ok || ref != "tmdb:603" {
		t.Fatalf("alias after reopen = (%q, %v, %v)", ref, ok, err)
	}
}
