# ADR 0001: Core language and runtime

- Status: Accepted
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #22
- Affects: Build 1 foundation, all services/connectors

## Context

The rewrite needs a default implementation language/runtime for the core service and built-in connectors, with a simple self-hosted deployment story.

## Decision

Keep **Go** as the initial core/runtime language for Build 1. Built-in connectors may run in-process in Go or out of process when isolation or language needs require it; the boundary is an implementation decision recorded when the connector mechanism is designed.

## Options considered

1. Go — simple deployment, strong service/tooling ecosystem, good fit for prior implementation.
2. TypeScript/Node — strong web ecosystem; different operational/runtime tradeoffs.
3. Rust — strong safety/performance; higher implementation cost for this stage.
4. Reopen per component — flexible, but adds early fragmentation.

## Consequences

Gives Build 1 a concrete starting point. Does not prevent out-of-process connectors in other languages. Exact module layout, frameworks, and versions are decided in foundation work, not frozen here.

## Follow-up

Confirm with #22; record repo layout and tooling pins in later ADRs/issues.
