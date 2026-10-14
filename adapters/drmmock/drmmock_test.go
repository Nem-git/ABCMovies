package drmmock

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/durationpb"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

func TestAcquireKeysReturnsCannedKeys(t *testing.T) {
	a := NewAdapter(nil)
	req := &slotsv1.AcquireKeysRequest{
		ProviderId: "jellyfin",
		ContentId:  "movie-1",
		System:     slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE,
		Pssh:       []byte("fake-pssh"),
		KeyIds:     []string{"key-1", "key-2"},
	}

	resp, err := a.AcquireKeys(context.Background(), req)
	if err != nil {
		t.Fatalf("AcquireKeys: %v", err)
	}
	if len(resp.GetKeys()) != 2 {
		t.Fatalf("got %d keys, want 2", len(resp.GetKeys()))
	}
	for i, k := range resp.GetKeys() {
		if k.GetKeyId() != req.KeyIds[i] {
			t.Errorf("key %d: got %q, want %q", i, k.GetKeyId(), req.KeyIds[i])
		}
		if len(k.GetKeyMaterial()) == 0 {
			t.Errorf("key %d: empty material", i)
		}
		if k.GetValidity().AsDuration() <= 0 {
			t.Errorf("key %d: non-positive validity", i)
		}
	}
}

func TestAcquireKeysMissingProvider(t *testing.T) {
	a := NewAdapter(nil)
	_, err := a.AcquireKeys(context.Background(), &slotsv1.AcquireKeysRequest{
		ContentId: "movie-1",
		System:    slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE,
		Pssh:      []byte("fake-pssh"),
		KeyIds:    []string{"key-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "provider_id") {
		t.Fatalf("expected provider_id error, got %v", err)
	}
}

func TestAcquireKeysMissingContentId(t *testing.T) {
	a := NewAdapter(nil)
	_, err := a.AcquireKeys(context.Background(), &slotsv1.AcquireKeysRequest{
		ProviderId: "jellyfin",
		System:     slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE,
		Pssh:       []byte("fake-pssh"),
		KeyIds:     []string{"key-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "content_id") {
		t.Fatalf("expected content_id error, got %v", err)
	}
}

func TestAcquireKeysMissingSystem(t *testing.T) {
	a := NewAdapter(nil)
	_, err := a.AcquireKeys(context.Background(), &slotsv1.AcquireKeysRequest{
		ProviderId: "jellyfin",
		ContentId:  "movie-1",
		Pssh:       []byte("fake-pssh"),
		KeyIds:     []string{"key-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "system") {
		t.Fatalf("expected system error, got %v", err)
	}
}

func TestAcquireKeysMissingPssh(t *testing.T) {
	a := NewAdapter(nil)
	_, err := a.AcquireKeys(context.Background(), &slotsv1.AcquireKeysRequest{
		ProviderId: "jellyfin",
		ContentId:  "movie-1",
		System:     slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE,
		KeyIds:     []string{"key-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "pssh") {
		t.Fatalf("expected pssh error, got %v", err)
	}
}

func TestAcquireKeysMissingKeyIds(t *testing.T) {
	a := NewAdapter(nil)
	_, err := a.AcquireKeys(context.Background(), &slotsv1.AcquireKeysRequest{
		ProviderId: "jellyfin",
		ContentId:  "movie-1",
		System:     slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE,
		Pssh:       []byte("fake-pssh"),
	})
	if err == nil || !strings.Contains(err.Error(), "key_id") {
		t.Fatalf("expected key_id error, got %v", err)
	}
}

func TestDecryptReturnsCannedClearBytes(t *testing.T) {
	a := NewAdapter(nil)
	req := &slotsv1.DecryptRequest{
		Source: &slotsv1.DecryptRequest_SourceRef{SourceRef: "ref-1"},
		Keys: []*slotsv1.ContentKey{
			{KeyId: "k1", KeyMaterial: []byte("mat"), Validity: durationpb.New(0)},
		},
	}

	resp, err := a.Decrypt(context.Background(), req)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(resp.GetClearBytes()) != "mock-clear-bytes" {
		t.Fatalf("clear bytes = %q, want mock-clear-bytes", resp.GetClearBytes())
	}
}

func TestDecryptMissingSourceRef(t *testing.T) {
	a := NewAdapter(nil)
	_, err := a.Decrypt(context.Background(), &slotsv1.DecryptRequest{
		Keys: []*slotsv1.ContentKey{{KeyId: "k1", KeyMaterial: []byte("m")}},
	})
	if err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("expected source error, got %v", err)
	}
}

func TestDecryptMissingKeys(t *testing.T) {
	a := NewAdapter(nil)
	_, err := a.Decrypt(context.Background(), &slotsv1.DecryptRequest{
		Source: &slotsv1.DecryptRequest_SourceRef{SourceRef: "ref-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "key") {
		t.Fatalf("expected key error, got %v", err)
	}
}
