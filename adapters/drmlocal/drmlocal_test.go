package drmlocal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nem-git/abcmovies/core/drm"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// TestSupports verifies the adapter reports Widevine support.
func TestSupports(t *testing.T) {
	a := &Adapter{}
	if !a.Supports(slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE) {
		t.Error("expected Widevine support")
	}
	if a.Supports(slotsv1.DrmSystem_DRM_SYSTEM_PLAYREADY) {
		t.Error("expected no PlayReady support")
	}
	if a.Supports(slotsv1.DrmSystem_DRM_SYSTEM_FAIRPLAY) {
		t.Error("expected no FairPlay support")
	}
}

// TestNewAdapterMissingPath verifies the adapter rejects an empty path.
func TestNewAdapterMissingPath(t *testing.T) {
	_, err := NewAdapter("", nil)
	if err == nil {
		t.Fatal("expected error for empty wvd_path")
	}
}

// TestNewAdapterNonExistentFile verifies the adapter rejects a missing file.
func TestNewAdapterNonExistentFile(t *testing.T) {
	_, err := NewAdapter("/nonexistent/path/to/device.wvd", nil)
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

// TestNewAdapterInvalidFile verifies the adapter rejects a corrupt .wvd file.
func TestNewAdapterInvalidFile(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.wvd")
	if err := os.WriteFile(badFile, []byte("not a real wvd file"), 0o600); err != nil {
		t.Fatalf("write bad wvd: %v", err)
	}
	_, err := NewAdapter(badFile, nil)
	if err == nil {
		t.Fatal("expected error for invalid wvd file")
	}
}

// TestNewAdapterEmptyFile verifies the adapter rejects an empty .wvd file.
func TestNewAdapterEmptyFile(t *testing.T) {
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.wvd")
	if err := os.WriteFile(emptyFile, []byte{}, 0o600); err != nil {
		t.Fatalf("write empty wvd: %v", err)
	}
	_, err := NewAdapter(emptyFile, nil)
	if err == nil {
		t.Fatal("expected error for empty wvd file")
	}
}

// TestAcquireKeysMissingPSSH verifies AcquireKeys rejects empty PSSH.
func TestAcquireKeysMissingPSSH(t *testing.T) {
	a := &Adapter{}
	_, err := a.AcquireKeys(context.Background(), "p", "c", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE, nil, []string{"kid"}, "https://license.example/", "auth")
	if err == nil {
		t.Fatal("expected error for empty pssh")
	}
}

// TestAcquireKeysMissingLicenseURL verifies AcquireKeys rejects empty license URL.
func TestAcquireKeysMissingLicenseURL(t *testing.T) {
	a := &Adapter{}
	_, err := a.AcquireKeys(context.Background(), "p", "c", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE, []byte("pssh"), []string{"kid"}, "", "auth")
	if err == nil {
		t.Fatal("expected error for empty license_url")
	}
}

// TestDecryptMissingSource verifies Decrypt rejects empty source.
func TestDecryptMissingSource(t *testing.T) {
	a := &Adapter{}
	_, err := a.Decrypt(context.Background(), drm.Source{}, nil)
	if err == nil {
		t.Fatal("expected error for empty source")
	}
}

// TestDecryptMissingKeys verifies Decrypt rejects empty keys.
func TestDecryptMissingKeys(t *testing.T) {
	a := &Adapter{}
	_, err := a.Decrypt(context.Background(), drm.Source{InlineBytes: []byte("data")}, nil)
	if err == nil {
		t.Fatal("expected error for empty keys")
	}
}

// TestAcquireKeysLicenseServerUnreachable verifies AcquireKeys returns an
// error when the license server is unreachable.
func TestAcquireKeysLicenseServerUnreachable(t *testing.T) {
	a := &Adapter{
		http: &http.Client{},
	}
	_, err := a.AcquireKeys(context.Background(), "p", "c", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE, []byte("pssh-data"), []string{"kid"}, "http://127.0.0.1:1/license", "auth")
	if err == nil {
		t.Fatal("expected error for unreachable license server")
	}
}

// TestAcquireKeysLicenseServer500 verifies AcquireKeys returns an error when
// the license server responds with a non-200 status.
func TestAcquireKeysLicenseServer500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	a := &Adapter{
		http: &http.Client{},
	}
	_, err := a.AcquireKeys(context.Background(), "p", "c", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE, []byte("pssh-data"), []string{"kid"}, srv.URL, "auth")
	if err == nil {
		t.Fatal("expected error for 500 license server")
	}
}
