# ADR 0010: Frontend/core split — deployable gRPC core and HTTP-facing frontends

- Status: Accepted
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #34, #23
- Affects: deployment packaging, web UI, HTTP API, core transport, identity contract

## Context

Implementation work surfaced that the browser-facing HTTP layer and the gRPC core were coupled in a single process, and that the contract mixed session handling with profile data. Two concerns were raised during review:

1. Users should be able to deploy **only the gRPC core**, and others **only a frontend**, with the two communicating over gRPC; the core must not serve HTTP.
2. HTTP-facing frontends repeatedly need the same session/cookie handling; repeating it per frontend is error-prone, especially once external frontends consume the HTTP API.

## Decision

- Split ABCMovies into **deployable units with a strict boundary**:
  - **Core** (`abcmovies-core`): gRPC only. No HTTP server, no cookie awareness. Owns the store (SQLite baseline, ADR 0004). Identity/session RPC surface lives here.
  - **HTTP API frontend** (`abcmovies-api`): JSON/HTTP API consumed by browser pages **and external frontends**. It is the **single HTTP session/cookie edge**: it maps the session cookie to core sessions (create/validate/destroy), and returns HTTP responses. It talks to the core over gRPC and holds no direct store access.
  - **SSR web frontend** (`abcmovies-web`): server-renders HTML using the standard library (`html/template` with embedded template files). It does **not** dial the core and does **not** own session logic; it calls the HTTP API server-side (forwarding cookies as needed) for session/subject resolution and proxies its own same-origin `/api/*` calls to the HTTP API.
- **gRPC transport between the HTTP frontend and core is insecure for now** (trusted/same-host assumption). Transport security for cross-host deployment is deferred to a future ADR; do not expose the core on an untrusted network in this state.
- **`user_id` is the stable unique identifier of a local user** everywhere (store keys, session subject, identifier fields in RPCs). `username` remains a login credential and display/profile attribute, not an identifier. (Glossary: "Account" is reserved for a user's access credentials/session for a source.)
- **Session/identity contract shape:** `Login` and `ValidateSession` return the identifier (`user_id`) only. Profile data is resolved via a dedicated `GetUser(user_id)` RPC rather than being embedded in session responses.
- `GetStatus` (`initialized: bool`) remains in `IdentityService` for now. `ListAccounts`/account management is deferred until account management is a real requirement.
- First-run setup (`SetupAdmin`) also establishes the initial session (`session_id` + `user_id`), so creating the admin behaves like registration — no separate sign-in step after setup. The HTTP API sets the session cookie itself.
- Build 1 keeps **stdlib `html/template` with embedded templates** for SSR; richer component approaches are deferred until the UI grows.

## Options considered

1. All-in-one binary (HTTP+gRPC in one process, browser talks to combined web layer) — simplest dev, but couples core to HTTP/browser concerns and prevents independent core/frontend deployment; rejected.
2. Two HTTP frontends both dial core over gRPC, sharing a session library — keeps everything over gRPC but still yields two session-aware HTTP edges (or shared library to maintain); rejected in favour of a single session edge given external-frontend consumption.
3. **Recommended (this ADR):** single HTTP session/cookie edge; other frontends delegate to it. SSR→API hop is HTTP; API→core hop is gRPC. Core remains gRPC-only and independently deployable.

## Consequences

- Three binaries with independent config; see `.env.example` (names only).
- Core and frontend can be deployed/run separately. The core cannot render pages; the web frontend cannot authenticate sessions by itself.
- Cross-network core/frontend deployment requires gRPC transport security before exposing beyond localhost/trusted networks.
- External frontends integrate against one stable HTTP API instead of reimplementing session handling.
- `user_id` is an opaque stable identifier; `username` may change without invalidating references/sessions.

## Follow-up

- gRPC transport security: revisit when cross-host deployment is targeted.
- Revisit frontend packaging (ADR 0009, still Proposed) and browser transport/gRPC-web (ADR 0003, still Proposed) as Build 1 grows.
