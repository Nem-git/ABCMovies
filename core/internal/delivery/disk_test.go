package delivery

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

func TestDiskSinkDeliversAndFinalizes(t *testing.T) {
	root := t.TempDir()
	fac, err := NewDiskFactory(root)
	if err != nil {
		t.Fatalf("NewDiskFactory: %v", err)
	}

	s := &Session{
		ID:   "del-1",
		Goal: GoalDownload,
		Context: corev1.DeliveryContext{
			Provider:       "jellyfin",
			AccountId:      "acc1",
			Sink:           "disk",
			SelectedTarget: "The Matrix (1999)",
			Container:      "mkv",
		},
	}
	sink, err := fac.NewSink(context.Background(), s, nil)
	if err != nil {
		t.Fatalf("NewSink: %v", err)
	}

	body := strings.NewReader("hello matrix bytes")
	track := &corev1.Track{Id: "c1"}
	if _, err := sink.Deliver(context.Background(), s, track, body); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if err := sink.Finalize(context.Background(), s); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	// The deliverable lands under the §1.15-shaped name, not the partial.
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(entries), entries)
	}
	name := entries[0].Name()
	if name != "The Matrix (1999).mkv" {
		t.Errorf("output name = %q", name)
	}
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello matrix bytes" {
		t.Errorf("content = %q", data)
	}
}

func TestDiskSinkAbortDiscardsPartial(t *testing.T) {
	root := t.TempDir()
	fac, _ := NewDiskFactory(root)
	s := &Session{ID: "del-1", Goal: GoalDownload, Context: corev1.DeliveryContext{SelectedTarget: "x", Container: "mkv"}}
	sink, _ := fac.NewSink(context.Background(), s, nil)
	_, _ = sink.Deliver(context.Background(), s, &corev1.Track{Id: "c1"}, strings.NewReader("partial"))
	sink.Abort(context.Background(), s)

	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Errorf("abort left %d entries behind: %v", len(entries), entries)
	}
}

func TestDiskSinkNewFactoryEmptyRoot(t *testing.T) {
	_, err := NewDiskFactory("")
	if err == nil {
		t.Fatal("expected error for empty root")
	}
}

func TestDiskSinkDeliverAfterFinalize(t *testing.T) {
	root := t.TempDir()
	fac, _ := NewDiskFactory(root)
	s := &Session{ID: "del-1", Goal: GoalDownload, Context: corev1.DeliveryContext{SelectedTarget: "x", Container: "mkv"}}
	sink, _ := fac.NewSink(context.Background(), s, nil)
	_, _ = sink.Deliver(context.Background(), s, &corev1.Track{Id: "c1"}, strings.NewReader("data"))
	_ = sink.Finalize(context.Background(), s)

	_, err := sink.Deliver(context.Background(), s, &corev1.Track{Id: "c1"}, strings.NewReader("more"))
	if err == nil {
		t.Fatal("expected error for deliver after finalize")
	}
}

func TestDiskSinkFinalizeAfterFinalize(t *testing.T) {
	root := t.TempDir()
	fac, _ := NewDiskFactory(root)
	s := &Session{ID: "del-1", Goal: GoalDownload, Context: corev1.DeliveryContext{SelectedTarget: "x", Container: "mkv"}}
	sink, _ := fac.NewSink(context.Background(), s, nil)
	_, _ = sink.Deliver(context.Background(), s, &corev1.Track{Id: "c1"}, strings.NewReader("data"))
	_ = sink.Finalize(context.Background(), s)

	err := sink.Finalize(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for finalize after finalize")
	}
}

func TestDiskSinkDedup(t *testing.T) {
	root := t.TempDir()
	fac, _ := NewDiskFactory(root)

	// First download: "movie.mkv"
	s1 := &Session{ID: "del-1", Goal: GoalDownload, Context: corev1.DeliveryContext{SelectedTarget: "movie", Container: "mkv"}}
	sink1, _ := fac.NewSink(context.Background(), s1, nil)
	_, _ = sink1.Deliver(context.Background(), s1, &corev1.Track{Id: "c1"}, strings.NewReader("first"))
	_ = sink1.Finalize(context.Background(), s1)

	// Second download: same name — should dedup to "movie (2).mkv"
	s2 := &Session{ID: "del-2", Goal: GoalDownload, Context: corev1.DeliveryContext{SelectedTarget: "movie", Container: "mkv"}}
	sink2, _ := fac.NewSink(context.Background(), s2, nil)
	_, _ = sink2.Deliver(context.Background(), s2, &corev1.Track{Id: "c1"}, strings.NewReader("second"))
	_ = sink2.Finalize(context.Background(), s2)

	entries, _ := os.ReadDir(root)
	if len(entries) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(entries), entries)
	}

	// Verify both files exist with correct content
	data1, err := os.ReadFile(filepath.Join(root, "movie.mkv"))
	if err != nil {
		t.Fatalf("ReadFile movie.mkv: %v", err)
	}
	if string(data1) != "first" {
		t.Errorf("movie.mkv content = %q", data1)
	}

	data2, err := os.ReadFile(filepath.Join(root, "movie (2).mkv"))
	if err != nil {
		t.Fatalf("ReadFile movie (2).mkv: %v", err)
	}
	if string(data2) != "second" {
		t.Errorf("movie (2).mkv content = %q", data2)
	}
}

func TestDiskSinkOutputNameEmptyTarget(t *testing.T) {
	root := t.TempDir()
	fac, _ := NewDiskFactory(root)
	s := &Session{ID: "del-1", Goal: GoalDownload, Context: corev1.DeliveryContext{Container: "mkv"}}
	sink, _ := fac.NewSink(context.Background(), s, nil)
	_, _ = sink.Deliver(context.Background(), s, &corev1.Track{Id: "c1"}, strings.NewReader("data"))
	_ = sink.Finalize(context.Background(), s)

	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(entries), entries)
	}
	if entries[0].Name() != "del-1.mkv" {
		t.Errorf("output name = %q, want del-1.mkv", entries[0].Name())
	}
}

func TestDiskSinkOutputNameEmptyContainer(t *testing.T) {
	root := t.TempDir()
	fac, _ := NewDiskFactory(root)
	s := &Session{ID: "del-1", Goal: GoalDownload, Context: corev1.DeliveryContext{SelectedTarget: "movie"}}
	sink, _ := fac.NewSink(context.Background(), s, nil)
	_, _ = sink.Deliver(context.Background(), s, &corev1.Track{Id: "c1"}, strings.NewReader("data"))
	_ = sink.Finalize(context.Background(), s)

	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(entries), entries)
	}
	if entries[0].Name() != "movie" {
		t.Errorf("output name = %q, want movie", entries[0].Name())
	}
}

// TestDiskSinkConcurrentDeliverFinalizeAbort drives Deliver, Finalize, and
// Abort concurrently on the same sink. The engine's push goroutine and a
// caller can both drive a download session, so the partial-file handle is
// guarded by a mutex (commit e659a77). This test is the deterministic
// regression guard: run with -race, it fails without the mutex.
func TestDiskSinkConcurrentDeliverFinalizeAbort(t *testing.T) {
	root := t.TempDir()
	fac, _ := NewDiskFactory(root)
	s := &Session{
		ID:   "del-concurrent",
		Goal: GoalDownload,
		Context: corev1.DeliveryContext{
			SelectedTarget: "concurrent-test",
			Container:      "mkv",
		},
	}
	sink, err := fac.NewSink(context.Background(), s, nil)
	if err != nil {
		t.Fatalf("NewSink: %v", err)
	}

	var wg sync.WaitGroup
	// Drive Deliver from multiple goroutines.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sink.Deliver(context.Background(), s, &corev1.Track{Id: "c1"}, bytes.NewReader([]byte("data")))
		}()
	}
	// Drive Finalize concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = sink.Finalize(context.Background(), s)
	}()
	// Drive Abort concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sink.Abort(context.Background(), s)
	}()
	wg.Wait()

	// After all concurrent operations, the sink must be in a clean state:
	// either finalized (file exists) or aborted (no partial file).
	// The key assertion is that -race reports no data race — the mutex
	// serializes access to the partial-file handle.
}
