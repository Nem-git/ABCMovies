# ADR 0002: Contract/schema format

- Status: Accepted
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #22
- Affects: API contracts, connector contracts, code generation

## Context

The product has internal/service boundaries (core, connectors, clients). The prior plan used schema-first contracts.

## Decision

Keep **Protocol Buffers** as the default contract/schema format for service and connector interfaces, with generated code where useful. Use a schema workflow/tooling choice (previously buf) as a foundation decision when contracts are introduced; do not add contract tooling before it is needed.

## Options considered

1. Protobuf — explicit contracts, code generation, good fit for prior API choice.
2. OpenAPI/JSON Schema — strong HTTP/browser/documentation fit; different streaming/contract model.
3. Language-native types only — fast initially; weak cross-boundary compatibility.

## Consequences

Supports typed contracts and future connector boundaries. Browser-facing transport may need an adapter (see ADR 0003). Exact schema layout and lint/breaking-change rules are deferred to the first contract milestone.

## Follow-up

Revisit when Build 1 needs its first stable API/connector contract; create a focused ADR for schema layout/versioning policy.
