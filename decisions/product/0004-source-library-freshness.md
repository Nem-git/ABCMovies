# ADR 0004: Source library freshness (Build 1, Jellyfin)

- Status: Accepted
- Type: Product
- Date: 2026-11-10
- Owners: TBD
- GitHub: #25
- Affects: REQ-LIB-04, REQ-LIB-05, #25

## Context

Build 1 syncs a connected Jellyfin source and presents its library. REQ-LIB-05 requires defining what “available” means per source kind and disclosing staleness/partial results. Sync uses transient credentials (no stored credentials/tokens) supplied at connect time or on manual re-sync.

## Constraints

- Honest, explainable availability; no silent guessing.
- No credentials/tokens persisted; transient credentials only.
- Build 1 single source (instance/household scope); multi-source/remote state deferred.

## Decision

- Treat Jellyfin as an **indexed source**: the library is a local snapshot refreshed by `SyncSource`.
- Record per-source sync state: `last_synced_at`, outcome (success/partial/failed), safe detail, and title count.
- **Availability (Build 1):** a title is available to present when it appears in the latest known snapshot of an active, reachable source. Titles absent after a successful sync are no longer presented as available/playable.
- If sync cannot complete (source unreachable or partial failure), keep the prior snapshot, mark the sync state as failed/partial and stale, and disclose it clearly; do not guess or silently present removed/unreachable titles as playable.
- The interface shows when the library was last synced and any partial/failed/stale state. Manual re-sync requires the user to provide credentials again (until a separate, approved custody decision changes this).

## Options considered

1. **A. Indexed snapshot with disclosed freshness (accepted)** — honest and simple; depends on user-initiated sync; no credential storage.
2. **B. On-demand per request** — always fresh but requires stored access and live calls; not feasible without credential custody and is heavier.
3. **C. Best-effort background sync** — needs stored credentials/tokens and a custody decision; deferred to a later decision.

## Consequences

- Honest, explainable availability for a single indexed source without storing credentials.
- Freshness depends on user-initiated sync; this is disclosed. Background/automatic refresh and credential custody are deferred.

## Follow-up

- Revisit when credential custody/background sync is decided, or when on-demand/non-indexed sources are added.

## Outcome

**Accepted** — indexed Jellyfin snapshot with disclosed freshness and explicit re-sync credentials for Build 1. Affected issues/docs: #25, DEC-003 (#18). Date: 2026-11-10. Owners: TBD (to be filled in implementation).
