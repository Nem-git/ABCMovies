# ADR 0006: Repository layout

- Status: Accepted
- Type: Technical
- Date: 2026-11-09
- Owners: TBD
- GitHub: #22
- Affects: developer workflow, CI, module boundaries

## Context

The reset removed the old layout. The rewrite needs a simple structure that can grow from docs/decisions to a working Build 1.

## Decision

Keep one repository and one Go module for Build 1. The three deployable entrypoints are `cmd/abcmovies-core`, `cmd/abcmovies-api`, and `cmd/abcmovies-web`. Keep each unit's implementation behind its own `internal` package so sibling units cannot import it:

- `core/` — public composition wrapper; `core/internal/` holds services, store, auth, audit, and connectors.
- `api/` — public composition wrapper; `api/internal/` holds the HTTP API and session edge.
- `web/` — public composition wrapper; `web/internal/` holds the SSR server and templates.

Generated contracts stay under `gen/go`, schemas under `proto`, and genuinely cross-unit helpers under `internal/` (for example `internal/platform` and `internal/config`). `internal/app` remains the thin composition root that re-exports the units' public constructors for binaries and end-to-end tests; it does not contain domain logic.

Do not split `core`, `api`, and `web` into separate repositories or projects yet. They are separate deployable units with a strict dependency direction, not independent codebases:

- `web` may call the HTTP API only; it must not import core or store internals.
- `api` may use generated core clients and HTTP/session code; it must not import core implementation or store internals.
- `core` owns the store, auth, audit, and connector internals; it must not import HTTP or web packages.
- `core/cmd/abcmovies-admin` is the operator-side core tool and may import `core/internal` directly.

The nested `internal` packages make the boundary compiler-enforced between units. Shared helpers remain importable only where genuinely cross-unit.

## Options considered

1. Minimal now, grow by slice — matches greenfield reset.
2. Restore old layout — familiar, but imports assumptions from removed implementation.
3. Monorepo with many packages immediately — flexible; premature overhead.
4. Separate repositories/modules for core, api, and web — rejected for Build 1: contract changes would need multi-repo coordination, generated code would need a shared release, and independent release cadence does not exist yet.

## Consequences

- One `make check`, one CI workflow, and one generated-code pipeline cover all three units.
- Protobuf changes can land atomically with core and API changes.
- The units can still be deployed and run separately, as required by the frontend/core split.
- A future split into separate modules or repositories should be revisited when independent releases, separate ownership, or stronger security/transport boundaries require it.

## Follow-up

- Revisit separate modules/repositories when independent release cadence or ownership boundaries exist.
