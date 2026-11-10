# ADR 0011: Store layout

- Status: Accepted
- Type: Technical
- Date: 2026-11-10
- Owners: TBD
- GitHub: #25
- Affects: core store, #25

## Context

The rewrite started with a single `store` file that owns schema and all domain access. Adding domains (sources → library) showed the structure needed explicit boundaries before it grows further.

## Decision

- SQLite implementation lives under `internal/core/internal/store/sqlite/` with one file per domain (`users.go`, `sessions.go`, `sources.go`, `library.go`).
- `internal/core/internal/store/store.go` is the package the rest of core imports; it re-exports the SQLite surface (types + errors + `Open` + mapped proto-free values).
- Schema migrates from one place in `store/sqlite/sqlite.go`; single database connection/owner remains.
- Domain contracts/interfaces stay minimal in Build 1 (facade/re-export); full ports-and-adapters is deferred until a second adapter/test seam justifies it.

## Options considered

A. One file, grow it — rejected (loses bounds).
B. Flat split without an implementation folder — rejected; SQLite code should sit behind one package boundary.
C. Full ports-and-adapters — deferred; revisit when a second adapter/test seam exists.

## Consequences

- Clear SQLite implementation folder, still one database/connection and one schema owner.
- Small mechanical refactor; low risk.
- Future extraction to stricter ports is possible without re-learning the layout.

## Outcome

**Accepted** — SQLite impl under `store/sqlite/`; `store` facade re-exports. Date: 2026-11-10. Owners: TBD.
