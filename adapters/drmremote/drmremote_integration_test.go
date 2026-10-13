package drmremote

import (
	"context"
	"encoding/base64"
	"os"
	"testing"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// TestIntegrationRemoteEndToEnd runs the full remote key acquisition flow
// against a real pywidevine service. It skips unless ABCMOVIES_PYWV_URL and
// ABCMOVIES_PYWV_DEVICE are set and -short is not passed.
//
// Usage:
//
//	ABCMOVIES_PYWV_URL=http://localhost:8080 ABCMOVIES_PYWV_DEVICE=device1 \
//	  go test -v ./adapters/drmremote/ -run TestIntegration
func TestIntegrationRemoteEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	serviceURL := os.Getenv("ABCMOVIES_PYWV_URL")
	if serviceURL == "" {
		t.Skip("skipping: ABCMOVIES_PYWV_URL not set")
	}
	deviceName := os.Getenv("ABCMOVIES_PYWV_DEVICE")
	if deviceName == "" {
		t.Skip("skipping: ABCMOVIES_PYWV_DEVICE not set")
	}
	secretKey := os.Getenv("ABCMOVIES_PYWV_SECRET")

	a, err := NewAdapter(serviceURL, secretKey, deviceName, nil)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}

	// PSSH from the Shaka Packager Widevine test stream.
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
	t.Logf("acquired %d keys from remote service at %s", len(keys), serviceURL)
	for _, k := range keys {
		t.Logf("  key_id=%s material_len=%d", k.GetKeyId(), len(k.GetKeyMaterial()))
	}
}

// TestIntegrationRemoteSupports verifies the adapter reports Widevine support.
func TestIntegrationRemoteSupports(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	serviceURL := os.Getenv("ABCMOVIES_PYWV_URL")
	if serviceURL == "" {
		t.Skip("skipping: ABCMOVIES_PYWV_URL not set")
	}
	deviceName := os.Getenv("ABCMOVIES_PYWV_DEVICE")
	if deviceName == "" {
		t.Skip("skipping: ABCMOVIES_PYWV_DEVICE not set")
	}

	a, err := NewAdapter(serviceURL, "", deviceName, nil)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if !a.Supports(slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE) {
		t.Error("expected Widevine support")
	}
}
