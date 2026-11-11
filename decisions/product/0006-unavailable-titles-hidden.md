# ADR 0006: Unavailable titles are hidden from browse/search (Build 1)

- Status: Accepted
- Type: Product
- Date: 2026-11-11
- Owners: TBD
- GitHub: #49 (implements); related #25, #47
- Affects: REQ-LIB-03, REQ-LIB-04, REQ-LIB-05, Build 1

## Context

Build 1 syncs an indexed Jellyfin library: a title absent after a successful sync is marked unavailable (never deleted) per ADR 0004. The shipped behavior listed unavailable titles in browse/search, labeled as unavailable, as an "honesty signal" for the interface. That answer was an implementation choice, not a recorded product decision. The operator's reading (confirmed in review) is that users do not want to be *shown* titles they cannot use in the catalog at all — the catalog is for finding things to watch.

REQ-LIB-03: visibility follows the user's reachable access; REQ-LIB-04: stop presenting unavailable items as playable and make unavailability clear. ADR 0004 decides *that* absent titles are marked unavailable; it does not say where unavailable titles may be shown.

## Decision

- **Browse and search list only available titles.** A title marked unavailable after a successful sync does not appear in the Titles list or in search results. No "N unavailable titles hidden" disclosure — the lists simply contain what is available and actionable.
- **Direct id resolution is unchanged.** A title marked unavailable stays resolvable by id (bookmarks, already-open pages): the detail page renders with the honest "not currently presented as playable" notice and no play form; a play attempt is rejected honestly (as today).
- **Failed/partial sync is untouched.** The prior snapshot remains listed and staleness is disclosed — unavailability is trustworthy only after a successful sync.
- Count/pagination reflect available titles only. Sync-state title counts are unaffected (they already count available titles).
- This mirrors the disconnect/tombstone rule (#47): hidden from browse/search, resolvable by id with the honest notice, store rows kept (future personal-state references stay meaningful).

## Options considered

1. **A. Hide from browse/search, keep by-id resolvable (accepted)** — smallest, most honest surface; nothing actionable is buried; matches the tombstone pattern; no "gone title" clutter.
2. **B. Show unavailable titles labeled as today** — the shipped behavior; kept an honesty signal but contradicts the catalog-is-for-watching purpose and REQ-LIB-03's visibility intent; rejected by the operator.
3. **C. Hide by default with a "show unavailable" affordance** — strictly more UI and test surface; no part of this was asked for; deferred indefinitely unless a real need appears.

## Consequences

- A title disappears from the lists after a successful sync that omits it. There is no inline explanation; the honest notice is reachable by direct id, and sync-state visibility ("not synced"/failed/partial) discloses unreliable snapshots.
- Store rows for unavailable titles are kept (never deleted) as today; browse/search simply filter them out.
- Detail pages for unavailable titles remain reachable by id — required for honest recovery messaging and for future saved/history references (Build 2 personal state).
- No change to availability marking on sync (ADR 0004 unchanged), to the play rejection path, or to logs/counts.

## Follow-up

- #49 implements this ADR.
- REQ-LIB-04 wording stays as-is; this ADR is the durable record for the visibility rule.
