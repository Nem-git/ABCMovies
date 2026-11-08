# ADR 0003: API and browser transport

- Status: Proposed (migration draft)
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #22
- Affects: Core API, web UI, auth/session handling

## Context

The core exposes an API used by the web UI first. Browsers have transport constraints that differ from native/service clients.

## Decision

Keep **gRPC** as the default core service contract style, and use a **browser-compatible adapter layer** (previously Connect/gRPC-Web style) owned by the web/frontend serving layer rather than making the browser talk to the core directly. Non-browser/internal clients may use the native service transport when appropriate.

## Options considered

1. gRPC core + browser adapter — preserves typed contracts; adds a serving/translation layer.
2. HTTP/JSON API only — simplest browser story; weaker fit with Protobuf/gRPC contracts.
3. Dual public APIs — flexible; high drift/maintenance risk.

## Consequences

The web UI can evolve without forcing browser transport concerns into the core. Authentication, CORS, streaming, and error mapping must be designed explicitly in the frontend adapter/foundation work.

## Follow-up

Confirm before Build 1 API foundation. Record exact libraries/protocols when implementation starts.
