# ADR 0009: Frontend packaging

- Status: Superseded by tech/0010
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #22
- Affects: web UI, deployment, browser transport

## Context

Build 1 is web-first. The prior plan served the browser through a frontend-owned serving/adapter layer while keeping the core focused on service APIs.

## Decision

Keep the **web UI as the first interface**. Package/serve it so the browser talks to a **frontend-owned web serving/adapter layer**, not directly to internal core transport. The exact deployment shape (single process, separate process, static assets plus adapter) is chosen in foundation/implementation, but the boundary should preserve browser compatibility and keep core transport concerns out of the browser.

## Options considered

1. Frontend adapter layer — clean browser boundary, matches ADR 0003.
2. Browser calls core directly — simpler topology; couples core to browser transport/CORS/auth concerns.
3. Separate unrelated frontend app from day one — flexible; premature for Build 1.

## Consequences

Build 1 can ship one usable web path without committing to the final packaging model. CORS/auth/streaming behavior belong in explicit design, not assumptions.

## Follow-up

Decide concrete packaging in foundation after #22 and ADR 0003 are accepted.
