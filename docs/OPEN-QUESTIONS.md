# Open questions / decisions needed

Unresolved choices the rewrite should settle (or consciously defer) and track in the project tool. Each item states the **question**, **why it matters**, and **options/tradeoffs**. None of these are decided here.

**Decided during the Build 1 work** (recorded; kept here for traceability — not open):

- **How are sources/servers identified?** → `decisions/product/0002-source-identity.md` (DEC-001, #15).
- **What is the stance on protected/sensitive content (e.g., DRM)?** → `decisions/product/0003-drm-protected-content-stance.md` (DEC-008, #17).
- **How are frontends packaged and shipped relative to the core service?** → `decisions/tech/0010-frontend-core-split.md` (#34); the `tech/0009` draft is marked Superseded.

## Identity & naming
- **How are extensions/integrations identified and trusted?**
  - *Why it matters:* Relates to installation, configuration, permissions, and preventing untrusted code from impersonating a trusted integration.
  - *Options / tradeoffs:*
    - Locally declared/managed integrations (simple, operator-trusted) vs. verifiable identities/signing/distribution (safer at scale, more complexity). Decide based on how extensions are packaged/loaded.
  - *Status:* open; nothing in Build 1 ships an extension surface. Decide together with "How are extensions defined, loaded, and isolated?" or the first extension that needs it.

## Library behavior
- **What evidence is required to merge two items as the same title, and how are conflicts handled?**
  - *Why it matters:* Wrong merges corrupt the library; over-separation creates duplicates. This is core to user trust.
  - *Options / tradeoffs:*
    - Prefer strong/authoritative identifiers when present; require corroboration for heuristic matches — safer, may leave duplicates.
    - Allow user correction/overrides later — flexible, but adds complexity and needs auditability.
    - *Non-binding idea:* keep conflicting/uncertain cases separate and visible rather than guessing; never fail silently.
  - *Status:* pending a multi-source world; **moot for Build 1** (single Jellyfin source — no cross-source merging is possible). Must be decided before a second source is added.

- **How is enrichment configured and scoped?**
  - *Why it matters:* Enrichment touches external services and user preferences (privacy, relevance, availability).
  - *Options / tradeoffs:*
    - Instance-level defaults with per-user overrides vs. per-user only — balances simplicity and control.
    - Opt-in vs. on-by-default — privacy-friendly vs. convenience; must remain disableable and non-blocking.
  - *Status:* open; enrichment is Later unless scheduled.

## Access, sharing & credentials
- **Where do credentials live and who can use them (custody/sharing model)?**
  - *Why it matters:* Drives security, privacy, and the feasibility of sharing access (“share use” without sharing secrets).
  - *Options / tradeoffs:*
    - Host-managed credentials (simpler UX) vs. user/owner-held credentials or delegated tokens (stronger boundaries, more moving parts).
    - Sharing models range from “grant use, host relays” to stricter delegation; each has different revocation and exposure properties.
    - *Non-binding idea:* pick the simplest model that meets the principles, document its trust boundaries honestly, and design for revocation.
  - *Status:* Build 1 interim for local use only: a per-user in-memory session credential cache (`decisions/product/0005-session-credential-cache.md`, #26) — credentials are never stored at rest. Durable custody and the sharing model are DEC-003, tracked in #18 (Build 2); decide before any sharing/delegation ships.

- **What does revocation guarantee, especially for in-progress activity?**
  - *Why it matters:* Users need predictable behavior when access is removed.
  - *Options / tradeoffs:*
    - Immediate stop of new and in-progress use (strong control, more complexity) vs. stop new use and let in-progress finish (simpler, weaker guarantee). Define and communicate clearly.
  - *Status:* DEC-004, tracked in #19 (Build 2). Until that lands, Build 1's cache revocation lag is bounded by the sliding window in ADR 0005; the stream ticket model already binds tickets to one title and expires on idle.

## Extensibility & packaging
- **How are extensions (sources/interfaces) defined, loaded, and isolated?**
  - *Why it matters:* Central to adding sources/interfaces safely without redesign.
  - *Options / tradeoffs:*
    - In-process plugins (simple, fast, weaker isolation) vs. out-of-process/services (stronger isolation, more operational overhead) vs. a hybrid. Choose against the isolation principle and operational cost.
    - *Non-binding idea:* a narrow, versioned interface/contract per extension type; validate inputs/outputs and fail closed.
  - *Status:* open; not scheduled.

## Tracking

This list mirrors the GitHub project. Closing a question happens by recording a decision (product/ or tech/), not by deleting an item; resolved items are struck from here with a pointer to the decision that resolved them.
