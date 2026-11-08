package e2e

import "testing"

// TestBuild1Journey walks the Build 1 end-to-end journey. Each step is
// skipped until the corresponding slice exists; this file exists so #27
// (end-to-end acceptance journey) has a stable entry point to fill in.
//
// Slices and decisions:
//  1. First-run admin + login          — REQ-IDN-01, DEC-009 (#23)
//  2. Connect/verify Jellyfin source   — REQ-SRC-01, DEC-001 (#24)
//  3. Sync/browse/search/title detail  — REQ-LIB-01/02/03 (#25)
//  4. Play reachable non-DRM title     — REQ-PLY-01, DEC-008 (#26)
func TestBuild1Journey(t *testing.T) {
	t.Run("first-run-admin-and-login", func(t *testing.T) {
		t.Skip("requires REQ-IDN-01 implementation (#23, DEC-009)")
	})
	t.Run("connect-and-verify-jellyfin", func(t *testing.T) {
		t.Skip("requires REQ-SRC-01 implementation (#24, DEC-001)")
	})
	t.Run("sync-browse-search-detail", func(t *testing.T) {
		t.Skip("requires REQ-LIB-01/02/03 implementation (#25)")
	})
	t.Run("play-reachable-non-drm-title", func(t *testing.T) {
		t.Skip("requires REQ-PLY-01 implementation (#26, DEC-008)")
	})
}