# ADR 0006: Repository layout

- Status: Proposed (migration draft)
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #22
- Affects: developer workflow, CI, module boundaries

## Context

The reset removed the old layout. The rewrite needs a simple structure that can grow from docs/decisions to a working Build 1.

## Decision

Start with the current top-level structure (`docs/`, `decisions/`, `.github/`, root docs). Add code directories only when foundation work begins, using clear boundaries such as contracts, core/service, connectors/integrations, web UI, and tests. Do not recreate the old full layout preemptively.

## Options considered

1. Minimal now, grow by slice — matches greenfield reset.
2. Restore old layout — familiar, but imports assumptions from removed implementation.
3. Monorepo with many packages immediately — flexible; premature overhead.

## Consequences

The first foundation PR should propose the concrete code layout as a reviewed change. `AGENTS.md` and protected-file rules apply.

## Follow-up

Draft layout in the Build 1 foundation issue before adding build tooling.
