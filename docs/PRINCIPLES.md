# Principles & constraints

Non-negotiable principles for the rewrite. They guide requirements and implementation choices. Where helpful, each includes **Ideas / options (non-binding)** to capture possible approaches — these are prompts for discussion, not commitments.

## 1) Aggregate legitimate access; no bundled secrets
- **Principle:** The system orchestrates a user’s own access to media sources. It does not provide content and does not ship with accounts, credentials, keys, or other secrets.
- **Constraint:** Secrets/credentials must never be committed to source control or exposed in logs/errors/UI. The instance may store user-supplied source credentials/session material to connect access; that material is sensitive and never bundled with the product.
- **Ideas / options (non-binding):**
  - Document clearly what configuration is secret vs. non-secret; provide examples/templates that contain no real secrets.
  - Treat any credential material as sensitive by default (redact in diagnostics).
  - Support connecting access through user-driven flows so the system stores only what’s necessary.

## 2) Least privilege and minimize what the host learns
- **Principle:** The system (and especially the host/core) should learn and retain as little as practical about users from the content and access it handles. Collect/keep only what’s needed to function.
- **Constraint:** Be explicit and honest about limits — some components must see requests/metadata to do their job; “privacy” is achieved by discipline and scoping, not assumed.
- **Ideas / options (non-binding):**
  - Separate concerns so components only receive the data they need (least-privilege data flow).
  - Avoid logging sensitive content/identifiers; prefer minimal, purpose-limited operational logs.
  - Make data retention (history, caches of sensitive material) configurable and bounded where feasible.

## 3) Extensions are not inherently trusted (isolation mindset)
- **Principle:** Source integrations and other extensions can be buggy or risky. The design should limit the blast radius of a misbehaving extension.
- **Constraint:** A failing or malicious extension must not be able to silently compromise unrelated users/data or take down the whole system by default.
- **Ideas / options (non-binding):**
  - Run extensions with least privilege and, where practical, stronger isolation boundaries (separate process/container/runtime) with explicit permissions.
  - Define a narrow, validated interface between the core and extensions; fail closed on invalid/unexpected behavior.
  - Attribute extension actions (for auditing/rate-limiting) and allow disabling/quarantining an extension.

## 4) Honest behavior over silent guessing
- **Principle:** The system must not mislead users. It should prefer clear, explainable outcomes over silent guesses that corrupt the library or hide failures.
- **Constraint:** Uncertain matches, missing data, and failures are surfaced clearly; nothing important fails silently.
- **Ideas / options (non-binding):**
  - Merge duplicates only with strong evidence; keep conflicts separate and visible rather than auto-merging on weak signals.
  - Show provenance/availability confidence where it helps users understand results.
  - Return actionable errors (what failed, what the user can do) without leaking secrets.

## 5) Per-user access and no dead ends
- **Principle:** What a user sees and can do is based on the access they actually have. Sharing grants use deliberately; private state stays private.
- **Constraint:** The UI/API must not routinely lead users to actions they cannot complete due to access; separation between users is enforced.
- **Ideas / options (non-binding):**
  - Compute/library views from reachable access; re-check or degrade gracefully when access changes.
  - Model sharing as explicit grants of *use* with revocation; define how revocation affects in-progress activity (a decision to make).
  - Scope personal data (history, saved items) per user by default.

## 6) Enrichment is helpful, never required
- **Principle:** External metadata can improve the experience but must remain optional and best-effort.
- **Constraint:** Core functions (identity, browse, playback, download, access control) work without enrichment; enrichment failures never block them.
- **Ideas / options (non-binding):**
  - Make enrichment sources/configuration opt-in or easily disabled.
  - Cache and tolerate missing/partial metadata; treat external data as untrusted input.

## 7) Protected/sensitive content handled explicitly and carefully
- **Principle:** Some sources involve protected or sensitive content/credentials. The rewrite must handle these cases deliberately, within clear legal and technical boundaries.
- **Constraint:** Lawfulness is the operator’s responsibility. The system documents its behavior/limits and avoids making unsafe or misleading promises; sensitive handling follows least privilege and no-secret-exposure rules.
- **Ideas / options (non-binding):**
  - Treat protected-content capabilities as explicit, isolatable components with strict boundaries (if supported at all).
  - Prefer approaches that work with authorized access and minimize sensitive material retained.
  - Record the project’s stance/limits as a resolved decision before implementing sensitive capabilities.

## 8) Keep decisions revisable
- **Principle:** Prefer clear, simple concepts and decisions that can be revisited. Capture options, then decide — don’t bake in accidental commitments.
- **Constraint:** Ideas are labeled non-binding until a decision is recorded. Process rules for docs (milestones/versions/one home per decision) belong in contribution/process docs, not as product behavior.
- **Ideas / options (non-binding):**
  - Track decisions as explicit items (problem, options, tradeoffs, outcome) in the project tracker.
  - Re-specify detailed contracts/schemas only when implementing a capability, informed by these principles.
