# ADR 0003: DRM/protected-content stance

- Status: Proposed (accepted for now; expected to be superseded later)
- Type: Product
- Date: 2026-11-08
- Owners: TBD
- GitHub: #17
- Affects: REQ-PLY-01, REQ-DL-01, REQ-SEC-01, docs/vision/requirements, Build 1/2 messaging

## Context

Playback and download can be over-promised when content is protected. The rewrite needs an explicit, honest stance before UI or implementation implies support.

## Constraints

- Lawfulness and operator responsibility are explicit.
- No misleading promises.
- Errors/UI must distinguish unavailable, access denied, unsupported, and protected/not supported where feasible.
- Least privilege; no secret exposure.
- Build 1 targets reachable **non-DRM** playback from Jellyfin.

## Decision

Accept **Option A for now**: protected/DRM content is **out of scope until explicitly decided**.

This decision is intentionally time-bounded in effect: it unblocks honest Build 1/2 behavior now and is **expected to be superseded** by a future ADR when the project deliberately addresses protected content. That future ADR must cover scope, legality/operator responsibility, technical approach, security boundaries, supported sources/clients, UX/error messaging, and testing/support limits.

### Build 1/2 behavior

- Support playback only for reachable **non-DRM** titles where the source and access permit.
- Do not claim DRM/protected-content support.
- When content cannot be played because it is protected or unsupported, show a clear, non-misleading message (specific where safely detectable; otherwise “cannot be played here: protected or unsupported content/format”).
- Apply the same honesty rule to downloads if/when downloads are scheduled.

## Options considered

1. **A. Protected content out of scope until explicitly decided (accepted for now)** — honest and simple; limits scope; may disappoint users with protected libraries.
2. **B. Best-effort pass-through only** — attempt playback when the path works without special DRM handling; can be ambiguous; must avoid claiming DRM support.
3. **C. Plan DRM capability now** — premature; legal/technical/security complexity blocks focus.

## Consequences

- Build 1 remains focused and honest.
- Some user libraries will contain titles that cannot be played; the UI must explain why without dead ends or false promises.
- Future DRM work is a major decision, not an incremental bug fix.

## Follow-up

- Reflect the non-DRM caveat in docs, issues, UI copy, and error handling.
- Add acceptance examples for protected/unsupported content to relevant issues.
- Supersede this ADR only with a dedicated protected-content decision.

## Outcome

**Accepted** (A now; expected to be superseded later) — protected/DRM content out of scope; Build 1/2 support reachable non-DRM titles only; clear non-misleading messages for protected/unsupported content. This decision is explicitly time-bounded and expected to be superseded by a future ADR when the project deliberately addresses protected content. Date: 2026-11-08. Owners: TBD (to be filled in implementation). Future superseding ADR will cover scope, legality/operator responsibility, technical approach, security boundaries, supported sources/clients, UX/error messaging, and testing/support limits. Affected issues/docs: #17, DEC-008 draft, foundation plan (#20/#21).
