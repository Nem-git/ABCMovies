package drmlocal

import (
	"context"
	"encoding/base64"
	"os"
	"testing"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// TestIntegrationAcquireKeys runs the full license acquisition flow against
// a real .wvd file and a public Widevine license server. It skips unless
// ABCMOVIES_WVD_PATH is set and -short is not passed.
//
// Usage:
//
//	ABCMOVIES_WVD_PATH=~/.config/abcmovies/device.wvd go test -v ./adapters/drmlocal/ -run TestIntegration
func TestIntegrationAcquireKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	wvdPath := os.Getenv("ABCMOVIES_WVD_PATH")
	if wvdPath == "" {
		t.Skip("skipping: ABCMOVIES_WVD_PATH not set")
	}

	a, err := NewAdapter(wvdPath, nil)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}

	// PSSH from the Shaka Packager Widevine test stream.
	// This is the init_data that the CDM needs to generate a license challenge.
	psshB64 := "AAAAW3Bzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAADsIARIQ62dqu8s0Xpa7z2FmMPGj2hoNd2lkZXZpbmVfdGVzdCIQZmtqM2xqYVNkZmFsa3IzaioCSEQyAA=="
	pssh, err := base64.StdEncoding.DecodeString(psshB64)
	if err != nil {
		t.Fatalf("decode pssh: %v", err)
	}

	// Public Widevine test license server (no auth required).
	licenseURL := "https://cwip-shaka-proxy.appspot.com/no_auth"

	keys, err := a.AcquireKeys(context.Background(), "test-provider", "test-content",
		slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE, pssh, []string{}, licenseURL, "")
	if err != nil {
		t.Fatalf("AcquireKeys: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("no keys acquired")
	}
	t.Logf("acquired %d keys", len(keys))
	for _, k := range keys {
		t.Logf("  key_id=%s material_len=%d", k.GetKeyId(), len(k.GetKeyMaterial()))
	}
}

// TestIntegrationSupports verifies the adapter reports Widevine support.
func TestIntegrationSupports(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	wvdPath := os.Getenv("ABCMOVIES_WVD_PATH")
	if wvdPath == "" {
		t.Skip("skipping: ABCMOVIES_WVD_PATH not set")
	}

	a, err := NewAdapter(wvdPath, nil)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if !a.Supports(slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE) {
		t.Error("expected Widevine support")
	}
}
