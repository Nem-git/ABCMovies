package delivery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
			MemberUserId:   "u1",
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
