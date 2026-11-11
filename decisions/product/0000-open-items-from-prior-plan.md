# ADR 0000: Open items carried from prior plan

- Status: Superseded
- Type: Product
- Date: 2026-11-08
- Owners: TBD
- GitHub: TBD
- Affects: DEC-001, DEC-005, DEC-006, DEC-007, extensibility, enrichment, frontend

## Context

These questions were noted before the rewrite reset. They are carried forward so they are not lost; each should become or be absorbed by a focused decision.

## Items

1. **Provider/source IDs** — How are provider IDs determined? A server URL alone can over- or under-distinguish (e.g. regional domains). IDs must be stable and not user-invented.
2. **Connector/extension IDs** — If sources/extensions are connectors, how are their IDs assigned and trusted?
3. **Enrichment scope** — Where is enrichment enabled/disabled (instance, source, user), and what is the default?
4. **Frontend shipping** — How are frontends shipped and deployed relative to the core service?

## Decision

No decision taken in this draft; the items were distributed instead — see Outcome.

## Outcome

**Superseded** — every item distributed (2026-11-11):

1. Provider/source IDs → `product/0002-source-identity.md` (DEC-001, #15): verified Jellyfin identity + opaque internal ID + normalized endpoint mapping for Build 1.
2. Connector/extension IDs → still open; tracked in `docs/OPEN-QUESTIONS.md` ("How are extensions/integrations identified and trusted?"). Not scheduled.
3. Enrichment scope → still open; tracked in `docs/OPEN-QUESTIONS.md` ("How is enrichment configured and scoped?"). Enrichment itself is Later unless scheduled.
4. Frontend shipping → `tech/0010-frontend-core-split.md` (#34): gRPC core + HTTP session edge; SSR web UI bundled with the core's HTTP edge. `tech/0009` records the superseded draft.

Further items surfaced by the rewrite to wait in `docs/OPEN-QUESTIONS.md`. This draft is closed.
