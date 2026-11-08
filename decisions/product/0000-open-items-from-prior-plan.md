# ADR 0000: Open items carried from prior plan

- Status: Proposed
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

No decision yet. Split into focused ADRs/issues (DEC-001, DEC-005, DEC-006, DEC-007).

## Follow-up

- File/link DEC-001, DEC-005, DEC-006, DEC-007.
- Supersede or close this draft when items are distributed.
