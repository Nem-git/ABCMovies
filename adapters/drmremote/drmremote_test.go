package drmremote

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nem-git/abcmovies/core/drm"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// mockPywidevineService implements the pywidevine serve API for testing.
type mockPywidevineService struct {
	t      *testing.T
	opened bool
	keys   []mockKey
}

type mockKey struct {
	KeyID string `json:"key_id"`
	Key   string `json:"key"`
	Type  string `json:"type"`
}

func (m *mockPywidevineService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check auth
	if r.Header.Get("X-Secret-Key") == "" && r.URL.Path != "/" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 401, "message": "Secret Key is Empty."})
		return
	}

	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/device1/open" {
			m.opened = true
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  200,
				"message": "Success",
				"data":    map[string]any{"session_id": "abc123"},
			})
			return
		}
	case http.MethodPost:
		switch r.URL.Path {
		case "/device1/get_license_challenge/STREAMING":
			if !m.opened {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"status": 400, "message": "No session"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  200,
				"message": "Success",
				"data":    map[string]any{"challenge_b64": base64.StdEncoding.EncodeToString([]byte("challenge"))},
			})
			return
		case "/device1/parse_license":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  200,
				"message": "Successfully parsed and loaded the Keys from the License message.",
			})
			return
		case "/device1/get_keys/ALL":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  200,
				"message": "Success",
				"data":    map[string]any{"keys": m.keys},
			})
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

// TestSupports verifies the adapter reports Widevine support.
func TestSupports(t *testing.T) {
	a := &Adapter{}
	if !a.Supports(slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE) {
		t.Error("expected Widevine support")
	}
	if a.Supports(slotsv1.DrmSystem_DRM_SYSTEM_PLAYREADY) {
		t.Error("expected no PlayReady support")
	}
}

// TestNewAdapterMissingServiceURL verifies the adapter rejects an empty service URL.
func TestNewAdapterMissingServiceURL(t *testing.T) {
	_, err := NewAdapter("", "secret", "device1", nil)
	if err == nil {
		t.Fatal("expected error for empty service_url")
	}
}

// TestNewAdapterMissingDeviceName verifies the adapter rejects an empty device name.
func TestNewAdapterMissingDeviceName(t *testing.T) {
	_, err := NewAdapter("http://localhost:8080", "secret", "", nil)
	if err == nil {
		t.Fatal("expected error for empty device_name")
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

// TestAcquireKeysEndToEnd verifies the full acquire flow against a mock
// pywidevine service.
func TestAcquireKeysEndToEnd(t *testing.T) {
	// Mock pywidevine service that returns hex-encoded keys (like the real service).
	srv := httptest.NewServer(&mockPywidevineService{
		t: t,
		keys: []mockKey{
			{KeyID: "6b696431", Key: "6b657931", Type: "CONTENT"}, // hex("kid1"), hex("key1")
		},
	})
	defer srv.Close()

	// Mock license server that accepts the challenge.
	licenseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock-license-response"))
	}))
	defer licenseSrv.Close()

	a, err := NewAdapter(srv.URL, "my-secret", "device1", nil)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}

	keys, err := a.AcquireKeys(context.Background(), "prov1", "content1", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE, []byte("pssh-data"), []string{"kid1"}, licenseSrv.URL, "auth")
	if err != nil {
		t.Fatalf("AcquireKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("got %d keys, want 1", len(keys))
	}
	if keys[0].GetKeyId() != "6b696431" {
		t.Errorf("key_id = %q, want 6b696431", keys[0].GetKeyId())
	}
}

// TestAcquireKeysServiceUnreachable verifies AcquireKeys returns an error
// when the service is unreachable.
func TestAcquireKeysServiceUnreachable(t *testing.T) {
	a, err := NewAdapter("http://127.0.0.1:1", "secret", "device1", nil)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}

	_, err = a.AcquireKeys(context.Background(), "p", "c", slotsv1.DrmSystem_DRM_SYSTEM_WIDEVINE, []byte("pssh"), []string{"kid"}, "https://license.example/", "auth")
	if err == nil {
		t.Fatal("expected error for unreachable service")
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
