# Requirements (capabilities)

IDs are stable references for issues and tests (`REQ-<AREA>-NN`). Items marked **Later** are documented but not in Build 1/2 unless scheduled.

Capabilities the rewrite needs to deliver. Each item states **intent** and **plain-language acceptance** (“done when…”). There is no phased roadmap here — ordering is handled in the project tracker.

## Library & discovery
- **REQ-LIB-01 Unified library**
  - *Intent:* Present one library entry per title across the sources a user can reach.
  - *Done when:* A title available through more than one reachable source appears once (where confidently the same title) and shows where it can be accessed; distinct titles are not wrongly combined.
- **REQ-LIB-02 Browse and search**
  - *Intent:* Let users find titles by browsing and by searching across their reachable sources.
  - *Done when:* A user can search/browse and get relevant results drawn from the sources they can access, with clear indication of availability.
- **REQ-LIB-03 Per-user view (no dead ends)**
  - *Intent:* A user’s library reflects only what they can actually access.
  - *Done when:* The catalog does not offer actions that fail solely because the user lacks access; visibility follows the user’s reachable access (including any access shared with them). Coverage/availability shown to the user reflects reachable sources only.
- **REQ-LIB-04 Availability changes handled honestly**
  - *Intent:* Content appears and disappears from sources over time.
  - *Done when:* The system updates availability, stops presenting unavailable items as playable, and makes unavailability clear. **Later (personal state):** if history/saved items exist, “gone everywhere” titles must not break those references; until then, do not assume personal state exists.

## Identity & merge
- **REQ-IDM-01 Confident identity, conservative merging**
  - *Intent:* Decide when two items from different sources are the same title without corrupting the library.
  - *Done when:* Merges rely on strong evidence; when evidence is weak or conflicting, items stay separate and the conflict can be surfaced rather than guessed. (See *Ideas / options* in PRINCIPLES and related backlog items.)
- **REQ-IDM-02 Stable references (Later where personal state is absent)**
  - *Intent:* A user’s references to a title (e.g., history, saved items) keep working as the library changes.
  - *Done when:* Normal library changes (re-merges, availability changes) do not silently orphan or mispoint a user’s saved references.

## Metadata & enrichment (Later unless scheduled)
- **REQ-ENR-01 Optional enrichment (Later unless scheduled)**
  - *Intent:* Add helpful metadata (artwork, descriptions, ratings, episode/series info where applicable) from external sources.
  - *Done when:* Enrichment improves display when available but is never required for browsing, identity, playback, or download; it can be turned off/configured, and missing enrichment degrades gracefully.

## Playback & streaming
- **REQ-PLY-01 Play from a reachable source**
  - *Intent:* Let a user watch a title through an interface, using a source they can access.
  - *Done when:* A user can start playback of an available, reachable, non-DRM title where the source and access permit it; the system selects/negotiates a compatible/playable form when feasible and reports clear errors when playback is not possible (protected, unavailable, unsupported, or access denied), with no silent failure.
- **REQ-PLY-02 Adapt to source and client needs (best effort)**
  - *Intent:* Sources provide media in different forms; clients have different capabilities.
  - *Done when:* The system delivers a compatible form when feasible (e.g., preferring a direct/compatible form, adapting when necessary) and is clear about limits or tradeoffs.

## Download (Later unless scheduled)
- **REQ-DL-01 Download a title (Later unless scheduled)**
  - *Intent:* Let a user save a title they can access to a destination they choose/control.
  - *Done when:* A user can request a download of an available, reachable, non-DRM title where the source and access permit it, choose reasonable options where applicable, and get a completed file or clear, actionable progress/failure status.

## Access, accounts & sharing
- **REQ-ACS-01 Connect and manage access**
  - *Intent:* Let users connect their own access to sources and manage it.
  - *Done when:* A user can add, verify, update, and remove their own source access; the system handles expired/invalid access clearly and prompts for re-authorization without exposing secrets.
- **REQ-ACS-02 Share use of access (controlled) (Build 2)**
  - *Intent:* Allow sharing the *use* of access with others without handing over ownership or secrets.
  - *Done when:* An owner can grant and revoke shared use; shared users gain access only as granted, and revocation takes effect in a clear, predictable way. The exact enforcement model is a decision (see OPEN-QUESTIONS / backlog).
- **REQ-ACS-03 Separation between users**
  - *Intent:* One user’s private state and access must not leak to another.
  - *Done when:* Access and any personal state are scoped correctly; one user cannot see or act on another’s private data or access beyond what was explicitly shared. Build 1/2 may have limited personal state; separation still applies to access, shares, and any state that exists.

## Jobs & status
- **REQ-JOB-01 Track long-running work**
  - *Intent:* Playback setup, downloads, and source/library refreshes take time and can fail.
  - *Done when:* Long-running work exposes status/progress, completes, fails with understandable reasons, and can be cancelled/stopped where appropriate; the state remains consistent after interruptions.

## Extensibility
- **REQ-EXT-01 Add sources and interfaces without redesign**
  - *Intent:* Support adding new media sources and new user interfaces over time.
  - *Done when:* Integrating a new source or a new interface follows a defined extension path and does not require reworking unrelated parts or compromising the principles below. The concrete extension mechanism is a decision (see OPEN-QUESTIONS / backlog).

## Identity, roles, and operator basics (to expand)

- **REQ-IDN-01 Local identity and roles (Build 1):** Admin/member/guest model with ownership as a relationship; first-run admin setup; login/session; no public signup by default unless decided.
- **REQ-OPS-01 Operator basics (Build 1/2):** Run/configure locally with documented steps and no secrets in examples. Build 2 release hygiene (install/upgrade/backup/restore, broader admin controls) is deferred; risk recorded.
- **REQ-SEC-01 Security baseline:** Secrets never shipped/exposed; user separation; honest access/protected-content limits; clear failures.
- **REQ-LIB-05 Source freshness (decision-backed):** Define what “available” means per source kind (indexed vs on-demand) and disclose staleness/partial results.

## Later (documented, not scheduled)

History/continue watching/saved items, downloads, enrichment, subtitle/multi-audio selection beyond pass-through, guests, live TV/recording, DRM/protected content, third-party extensions, extra clients, public signup, 2FA/SSO, telemetry.
