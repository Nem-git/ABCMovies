// Package app wires the ABCMovies deployable units together.
// Three independent units exist by design (see decisions/tech/0010):
//   - Core: gRPC-only service backed by SQLite. No HTTP, no cookies.
//   - API:  HTTP edge; the single place session/cookie semantics live.
//   - Web:  SSR HTML frontend; calls the API server-side; no session logic.
package app
