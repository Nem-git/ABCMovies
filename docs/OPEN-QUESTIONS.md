# Open questions / decisions needed

Unresolved choices the rewrite should settle (or consciously defer) and track in the project tool. Each item states the **question**, **why it matters**, and **options/tradeoffs**. None of these are decided here.

## Identity & naming
- **How are sources/servers identified (stable, not user-invented)?**
  - *Why it matters:* A stable identity is needed for configuration, caching, merge provenance, and permissions. User-chosen IDs drift; naive URL-based IDs can over- or under-distinguish (e.g., the same service reached via different regional domains/endpoints).
  - *Options / tradeoffs:*
    - Derive a canonical identity from a verified/normalized server endpoint — stable and automatic, but needs clear normalization and a story for aliases/region variants.
    - Use an operator-assigned internal ID mapped to verified endpoints — explicit, but adds management overhead and migration concerns.
    - *Non-binding idea:* whatever the scheme, make it deterministic, collision-resistant in practice, and not dependent on free-form user input.

- **How are extensions/integrations identified and trusted?**
  - *Why it matters:* Relates to installation, configuration, permissions, and preventing untrusted code from impersonating a trusted integration.
  - *Options / tradeoffs:*
    - Locally declared/managed integrations (simple, operator-trusted) vs. verifiable identities/signing/distribution (safer at scale, more complexity). Decide based on how extensions are packaged/loaded.

## Library behavior
- **What evidence is required to merge two items as the same title, and how are conflicts handled?**
  - *Why it matters:* Wrong merges corrupt the library; over-separation creates duplicates. This is core to user trust.
  - *Options / tradeoffs:*
    - Prefer strong/authoritative identifiers when present; require corroboration for heuristic matches — safer, may leave duplicates.
    - Allow user correction/overrides later — flexible, but adds complexity and needs auditability.
    - *Non-binding idea:* keep conflicting/uncertain cases separate and visible rather than guessing; never fail silently.

- **How is enrichment configured and scoped?**
  - *Why it matters:* Enrichment touches external services and user preferences (privacy, relevance, availability).
  - *Options / tradeoffs:*
    - Instance-level defaults with per-user overrides vs. per-user only — balances simplicity and control.
    - Opt-in vs. on-by-default — privacy-friendly vs. convenience; must remain disableable and non-blocking.

## Access, sharing & credentials
- **Where do credentials live and who can use them (custody/sharing model)?**
  - *Why it matters:* Drives security, privacy, and the feasibility of sharing access (“share use” without sharing secrets).
  - *Options / tradeoffs:*
    - Host-managed credentials (simpler UX) vs. user/owner-held credentials or delegated tokens (stronger boundaries, more moving parts).
    - Sharing models range from “grant use, host relays” to stricter delegation; each has different revocation and exposure properties.
    - *Non-binding idea:* pick the simplest model that meets the principles, document its trust boundaries honestly, and design for revocation.

- **What does revocation guarantee, especially for in-progress activity?**
  - *Why it matters:* Users need predictable behavior when access is removed.
  - *Options / tradeoffs:*
    - Immediate stop of new and in-progress use (strong control, more complexity) vs. stop new use and let in-progress finish (simpler, weaker guarantee). Define and communicate clearly.

## Delivery & sensitive content
- **What is the project’s stance on protected/sensitive content (e.g., DRM) in the rewrite?**
  - *Why it matters:* Has legal, security, and complexity implications; must align with “honest boundaries” and “least privilege.”
  - *Options / tradeoffs:*
    - Support only sources/content that work with straightforward, authorized access; treat protected-content capabilities as out of scope until explicitly decided.
    - If ever considered, isolate it strictly, minimize sensitive material retained, and document limits. *Non-binding idea:* make this an explicit, recorded decision before any implementation work.

## Extensibility & packaging
- **How are extensions (sources/interfaces) defined, loaded, and isolated?**
  - *Why it matters:* Central to adding sources/interfaces safely without redesign.
  - *Options / tradeoffs:*
    - In-process plugins (simple, fast, weaker isolation) vs. out-of-process/services (stronger isolation, more operational overhead) vs. a hybrid. Choose against the isolation principle and operational cost.
    - *Non-binding idea:* a narrow, versioned interface/contract per extension type; validate inputs/outputs and fail closed.

- **How are frontends packaged and shipped relative to the core service?**
  - *Why it matters:* Affects deployment, updates, and how independently interfaces can evolve (e.g., a web UI bundled with the core vs. delivered/run separately).
  - *Options / tradeoffs:*
    - Bundled with the core service (simpler all-in-one setup) vs. separate packages/services that talk to the core (independent release/update, supports multiple or third-party interfaces; more moving parts).
    - *Non-binding idea:* keep the core/interface boundary clear so either packaging model remains possible; decide based on target users’ deployment expectations.
