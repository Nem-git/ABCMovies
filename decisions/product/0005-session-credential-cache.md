# ADR 0005: Session credential cache (Build 1)

- Status: Accepted
- Type: Product
- Date: 2026-11-10
- Owners: TBD
- GitHub: #26 (implements; see also DEC-003 #18)
- Supersedes: ADR 0004, the clause "Manual re-sync requires the user to provide credentials again (until a separate, approved custody decision changes this)" in its Decision section only
- Affects: REQ-LIB-05, REQ-PLY-01, REQ-PLY-02, DEC-003, Build 1

## Context

Build 1 (Jellyfin, single source, no stored credentials) syncs an indexed library and plays reachable non-DRM titles through the web UI. Playback mints a short-lived stream ticket at play-time; minting requires source credentials. Making the user re-enter credentials on every sync, every play, and every re-sync would defeat REQ-PLY-01's purpose; holding credentials at rest is undecided (DEC-003, Build 2). The authorization happens at mint; nothing about the ticket or stream carries the credentials themselves.

## Decision

- Keep a **per-user, in-memory-only credential cache** on the core process for Build 1:
  - keyed by `user_id`; **shared by that user's concurrent sessions** (it is not per browser session);
  - **sliding 12-hour expiry**, refreshed on every read; no absolute cap while in active use;
  - contents are **never written to the store, logged, or returned to browsers**; cleared on process restart; **logout does not clear** them;
  - credentials are cached only after a successful authentication against the source; a **blank-credentials request reuses the cache**; with an empty cache the call fails with FailedPrecondition and the UI asks for credentials.
- This narrowly supersedes ADR 0004's manual re-sync credentials clause **for the duration of Build 1 only**, until DEC-003 decides custody. Everything else in ADR 0004 stands.
- The credential-resolution seam is the single place that changes when DEC-003 lands; the playback ticket model (actor- and title-scoped, 30-minute idle sliding capability token in the stream URL, uniform 404 for expired/foreign/wrong-title tickets) is independent of that future change.
- **REQ-PLY-02 known limit:** Build 1 plays the **original file as stored on the source** (direct play). No codec negotiation or transcoding is performed; clients that cannot decode the source's container/codecs (e.g. HEVC, AC3/DTS audio, MKV) fail via the honest error path. Adaptation beyond this is deferred, not designed, not promised.

## Options considered

1. **A. Per-user in-memory session cache (accepted)** — honest, bounded, no at-rest material; loses everything on restart; shared across the user's sessions; revocation lag bounded by the sliding 12h window plus mint-time verification.
2. **B. Credentials required on every sync/play** — maximal hygiene, unusable playback UX; contradicts REQ-PLY-01's goal.
3. **C. Store credentials at rest now (encrypted)** — premature while DEC-003 is open; implementation would be making a custody decision unilaterally.
4. **D. External secret store / vault integration now** — same shape of problem as C; deferred to DEC-003.

## Consequences

- After a restart the cache is empty: the user re-enters source credentials once on the next sync or play.
- Changing/removing credentials on the source does not evict a cached entry; the 12h sliding window applies on a per-entry basis, and API keys are verified at mint time (a stale key fails as 401, an honest error).
- Logout leaves cached credentials in process memory until the window lapses or the process restarts; this is disclosed here.
- Ticket model and item-scoping unchanged; no secrets enter URLs or the browser beyond the short-lived ticket.
- This ADR, not docs, is the place REQ-PLY-02's direct-play limit is recorded.

## Follow-up

- DEC-003 (Build 2, #18) decides at-rest custody; this cache is the seam it replaces.
- #26 / PR #44 implements this ADR.
- OPERATIONS.md records the operator-facing expectations.
- ADR 0004 carries a "Superseded by 0005" marker for the affected clause.
