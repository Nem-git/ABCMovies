// Package e2e holds end-to-end/acceptance tests for the Build 1 journey.
//
// The journey mirrors the Build 1 vertical slice: first-run admin setup and
// login, connect/verify a Jellyfin source, sync/browse/search/detail, and
// play a reachable non-DRM title. Tests here are expected to exercise the
// public/runtime path (HTTP/GRPC) rather than internal units.
package e2e