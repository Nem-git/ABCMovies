# Backlog (draft) — GitHub Issues & Project

Draft issues derived from the rewrite spec, ready to review and file in GitHub (Issues + a Project). Wording is milestone/version-free. **Ideas / options are non-binding** unless a decision issue is resolved and recorded.

Suggested labels: `area:library`, `area:identity-merge`, `area:enrichment`, `area:delivery`, `area:access-sharing`, `area:extensibility`, `area:security-privacy`, `type:decision`, `type:principle`, `type:capability`.

---

## Principles / reference (file as reference issues or link to PRINCIPLES.md)
- **[principle] No bundled secrets; safe credential handling** (`type:principle`, `area:security-privacy`)
  - *Context:* The system orchestrates legitimate access; it never ships secrets.
  - *Acceptance:* No secrets in source control; secrets redacted from logs/errors/UI; configuration templates contain no real secrets.
  - *Ideas / options (non-binding):* user-driven connect flows storing only what’s necessary; document secret vs non-secret config; treat credential material as sensitive by default.
- **[principle] Least privilege & minimize host knowledge** (`type:principle`, `area:security-privacy`)
  - *Context:* Limit what the host/core learns and retains from content/access.
  - *Acceptance:* Data collection/retention is purpose-limited; sensitive content/identifiers not logged; trust boundaries stated honestly.
  - *Ideas / options (non-binding):* least-privilege data flow between components; bounded/configurable retention for history/caches of sensitive material.
- **[principle] Extensions isolated; fail closed** (`type:principle`, `area:extensibility`, `area:security-privacy`)
  - *Context:* Extensions are not inherently trusted.
  - *Acceptance:* A misbehaving extension cannot silently compromise unrelated users/data or take down the whole system by default; invalid behavior fails closed.
  - *Ideas / options (non-binding):* stronger isolation boundaries (process/container/runtime) with explicit permissions; narrow validated interface; attribute actions; allow disable/quarantine.
- **[principle] Honest behavior, no silent guessing** (`type:principle`, `area:library`, `area:identity-merge`)
  - *Context:* User trust depends on explainable outcomes.
  - *Acceptance:* Uncertain matches, missing data, and failures are surfaced clearly; nothing important fails silently.
  - *Ideas / options (non-binding):* show provenance/confidence where helpful; actionable errors without leaking secrets.
- **[principle] Per-user access, no dead ends** (`type:principle`, `area:access-sharing`, `area:library`)
  - *Context:* Users see and can do only what their access allows.
  - *Acceptance:* No routine dead-end actions; sharing is explicit; private state scoped per user.
  - *Ideas / options (non-binding):* compute views from reachable access; graceful handling when access changes.
- **[principle] Enrichment optional and best-effort** (`type:principle`, `area:enrichment`)
  - *Context:* External metadata helps but must not be a dependency.
  - *Acceptance:* Core functions work without enrichment; enrichment can be disabled; failures never block core functions.
  - *Ideas / options (non-binding):* cache/tolerate partial metadata; treat external data as untrusted input.

## Decisions (file early; resolve and record outcomes)
- **[decision] Source/server identity scheme (stable, not user-invented)** (`type:decision`, `area:extensibility`, `area:library`)
  - *Problem:* Stable identity needed for config, caching, merge provenance, permissions; user-chosen IDs drift; URL-based IDs can mis-handle regional/alias endpoints.
  - *Acceptance:* A documented, deterministic identity scheme that is not free-form user input and handles endpoint normalization/aliases.
  - *Options / tradeoffs (non-binding):* canonical identity from verified/normalized endpoint (automatic; needs normalization/alias story) vs operator-assigned internal ID mapped to verified endpoints (explicit; management/migration overhead).
- **[decision] Extension identity & trust model** (`type:decision`, `area:extensibility`, `area:security-privacy`)
  - *Problem:* How integrations are identified/trusted affects permissions and impersonation risk.
  - *Acceptance:* A documented model for identifying extensions and the trust level assumed.
  - *Options / tradeoffs (non-binding):* locally declared/operator-trusted (simple) vs verifiable identity/signing/distribution (safer at scale, more complex).
- **[decision] Merge evidence & conflict handling** (`type:decision`, `area:identity-merge`, `area:library`)
  - *Problem:* Wrong merges corrupt the library; over-separation duplicates it.
  - *Acceptance:* Defined rules for when items merge vs stay separate, and how conflicts/uncertainty are surfaced; behavior is explainable.
  - *Options / tradeoffs (non-binding):* prefer strong authoritative IDs; require corroboration for heuristic matches; keep uncertain cases separate/visible; consider later user correction with auditability.
- **[decision] Enrichment configuration & scope** (`type:decision`, `area:enrichment`, `area:security-privacy`)
  - *Problem:* External enrichment affects privacy, relevance, and control.
  - *Acceptance:* Defined where enrichment is enabled/disabled and how it’s scoped (instance/user), consistent with “optional & best-effort.”
  - *Options / tradeoffs (non-binding):* instance defaults with per-user overrides vs per-user only; opt-in vs default-on (must remain disableable/non-blocking).
- **[decision] Credential custody & sharing model** (`type:decision`, `area:access-sharing`, `area:security-privacy`)
  - *Problem:* Sharing “use” without sharing secrets depends on where credentials live and how access is delegated/relayed.
  - *Acceptance:* A documented custody/sharing model with stated trust boundaries and revocation behavior.
  - *Options / tradeoffs (non-binding):* host-managed (simpler UX) vs user/owner-held or delegated tokens (stronger boundaries, more complexity); choose simplest model that meets principles and supports revocation.
- **[decision] Revocation semantics (incl. in-progress activity)** (`type:decision`, `area:access-sharing`)
  - *Problem:* Users need predictable behavior when access is removed.
  - *Acceptance:* Defined, documented behavior for revoking shared/use access, including impact on in-progress activity.
  - *Options / tradeoffs (non-binding):* stop new + in-progress immediately (strong control, complex) vs stop new and let in-progress finish (simpler, weaker). Communicate clearly.
- **[decision] Stance on protected/sensitive content (e.g., DRM)** (`type:decision`, `area:delivery`, `area:security-privacy`)
  - *Problem:* Legal, security, and complexity implications; must align with honest boundaries and least privilege.
  - *Acceptance:* An explicit, recorded stance/limits for the rewrite before any related implementation.
  - *Options / tradeoffs (non-binding):* support only straightforward authorized access; treat protected-content capabilities as out of scope until explicitly decided; if ever considered, isolate strictly and minimize sensitive material retained.
- **[decision] Extension mechanism & isolation level** (`type:decision`, `area:extensibility`, `area:security-privacy`)
  - *Problem:* How sources/interfaces plug in determines safety and operability.
  - *Acceptance:* A defined extension mechanism (interface/contract shape at a high level) and isolation expectations per extension type.
  - *Options / tradeoffs (non-binding):* in-process (simple, weaker isolation) vs out-of-process/services (stronger isolation, overhead) vs hybrid; narrow versioned interface; validate and fail closed.
- **[decision] Frontend packaging/shipping relative to core** (`type:decision`, `area:extensibility`, `area:delivery`)
  - *Problem:* Affects deployment, updates, and independent evolution of interfaces.
  - *Acceptance:* A documented packaging/deployment approach for frontends vs the core service.
  - *Options / tradeoffs (non-binding):* bundled with core (simpler all-in-one) vs separate packages/services talking to the core (independent release, supports multiple/third-party interfaces; more moving parts); keep the core/interface boundary clear so either remains possible.

## Capabilities
- **[capability] Unified library across reachable sources** (`type:capability`, `area:library`, `area:identity-merge`)
  - *Acceptance:* Same title across reachable sources appears once when confidently the same; distinct titles not wrongly combined; shows where accessible.
  - *Ideas / options (non-binding):* stable internal reference per title; map source items to it; merge on strong evidence; surface conflicts (see merge decision).
- **[capability] Browse and search across reachable sources** (`type:capability`, `area:library`)
  - *Acceptance:* Users can browse/search and get relevant results from sources they can access, with clear availability.
  - *Ideas / options (non-binding):* query sources in parallel where appropriate; degrade gracefully per-source; make relevance/coverage understandable.
- **[capability] Per-user library view with no dead ends** (`type:capability`, `area:library`, `area:access-sharing`)
  - *Acceptance:* Catalog reflects only accessible content; actions don’t routinely fail due to lack of access; visibility follows reachable/shared access.
  - *Ideas / options (non-binding):* derive views from reachable access; re-evaluate on access changes.
- **[capability] Handle availability changes honestly** (`type:capability`, `area:library`)
  - *Acceptance:* Availability updates; unavailable items not presented as playable; “gone everywhere” items don’t break existing references (e.g., history) and show as unavailable.
  - *Ideas / options (non-binding):* confirm availability at point of use; cache with clear freshness expectations; keep user references resolvable.
- **[capability] Optional, non-blocking metadata enrichment** (`type:capability`, `area:enrichment`)
  - *Acceptance:* Enrichment improves display when available; never required for core functions; configurable/disableable; failures degrade gracefully.
  - *Ideas / options (non-binding):* multiple enrichment sources possible; cache results; treat external data as untrusted.
- **[capability] Play available titles with clear errors** (`type:capability`, `area:delivery`)
  - *Acceptance:* User can start playback of an available, authorized title; system negotiates a compatible form when feasible; failures are clear and actionable, never silent.
  - *Ideas / options (non-binding):* prefer direct/compatible delivery; adapt when necessary; report limits/tradeoffs.
- **[capability] Download available titles with status** (`type:capability`, `area:delivery`)
  - *Acceptance:* User can request a download with reasonable options; receives a completed file or clear progress/failure.
  - *Ideas / options (non-binding):* expose progress/cancel; choose compatible output; handle interruptions predictably.
- **[capability] Connect and manage source access** (`type:capability`, `area:access-sharing`, `area:security-privacy`)
  - *Acceptance:* User can add/verify/update/remove their own access; expired/invalid access handled clearly with re-auth prompts; no secret exposure.
  - *Ideas / options (non-binding):* user-driven connect flows; validate access before storing; least data retained.
- **[capability] Share use of access with revocation** (`type:capability`, `area:access-sharing`, `area:security-privacy`)
  - *Acceptance:* Owner can grant/revoke shared use; shared users gain only granted access; revocation behaves per the recorded decision.
  - *Ideas / options (non-binding):* explicit grants of use (not ownership/secrets); define in-progress behavior (decision).
- **[capability] User separation & private state** (`type:capability`, `area:access-sharing`, `area:security-privacy`)
  - *Acceptance:* Personal state (history/saved items) and access scoped per user; no cross-user leakage beyond explicit sharing.
  - *Ideas / options (non-binding):* per-user scoping by default; audit-sensitive actions where appropriate.
- **[capability] Track long-running work (status/progress/cancel)** (`type:capability`, `area:delivery`, `area:library`)
  - *Acceptance:* Long-running work exposes status/progress, completes or fails with understandable reasons, supports cancel/stop where appropriate, and stays consistent after interruptions.
  - *Ideas / options (non-binding):* uniform job/status model across refresh, playback setup, and downloads.
- **[capability] Extensibility path for new sources and interfaces** (`type:capability`, `area:extensibility`)
  - *Acceptance:* A defined path to add a source or interface without reworking unrelated parts or violating principles (esp. isolation, honesty, least privilege).
  - *Ideas / options (non-binding):* narrow contracts per extension type; conformance/validation checks; keep core/interface boundary clear (see extension + frontend decisions).
