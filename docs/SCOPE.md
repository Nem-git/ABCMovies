# Scope (v1)

This document fixes **what v1 of the project is**: which milestones ship, what is explicitly out, what "v1 done" means, and the operator sign-offs the plan requires (lawfulness in particular). It exists so IMPLEMENTATION.md's milestone roadmap stays a generic reference while the actual release boundary lives here. PLAN.md remains the spec; this document is a commitment about *release*, not a change to what the system is.

**Boundary:** product decisions → PLAN.md §11; implementation decisions → TECHNICAL-DECISIONS.md; scope and acceptance → here.

## 1. v1 boundary

**v1 = milestones M0–M6** (IMPLEMENTATION.md §3).

| In v1 | Milestones | What it delivers |
| --- | --- | --- |
| M0 | Walking skeleton | Registry, one in-process slot, store classes, API server, one sync call + one event to a web client |
| M1 | Library-class provider adapter | The first library-class provider (TECHNICAL-DECISIONS.md §1.8), whole-catalogue sync, account source cache |
| M2 | Library + matching + merge | LibraryEntry, provider item registry, per-user merged library |
| M3 | Enrichment | Catalogue slot, field-level merging, provenance per external ID |
| M4 | Delivery engine | MediaSource manifest, passthrough play + remux download end-to-end, heartbeat/TTL/revocation; **two co-equal v1 sinks** — the user's device (built-in, browser download) and instance-local disk (TECHNICAL-DECISIONS.md §1.13) |
| M5 | First frontend | Web frontend on the core API; one inbound protocol locked (TECHNICAL-DECISIONS.md §1.2) |
| M6 | Sharing + policy + audit | Account sharing by use, min(policy, cap), revocation, member-scoping, account-scoped availability events |

**Explicitly out of v1 (deferred to v2):**

| Deferred | Where it's specified | Note |
| --- | --- | --- |
| DRM slot (acquire-keys / decrypt) | PLAN.md §6.6 | See sign-off in §3 |
| Lazy streaming-service providers | PLAN.md §5.4 | v1 providers are library-class only |
| Sidecar custody | PLAN.md §7.4 | Server-held custody only in v1; relay-through-owner is v2 |
| Egress routing | PLAN.md §7.3 | Off by default; not built in v1 |
| Streaming-service pacing integration | PLAN.md §5.4, §7.2 | Pacing machinery is built (M6/M7); the lazy-provider consumer is v2 |
| Scoring profiles, session coalescing, entry-level correction, license-wrapper composition, direct-URL handoff, two-factor / identity-provider login | PLAN.md §6.3, §6.5, §5.3, §6.6, §7.6 | Already documented as future/deferred in IMPLEMENTATION.md §3 |
| Guests (`guest:<deviceId>` library cache) | PLAN.md §2.2, §5.1 | No guest concept in v1; the guest device cache is v2 |
| User slots (attribution, tenancy scope, transport restriction, revocability) | PLAN.md §4, §4.1 | v1 ships account sharing by use only (M6); user slots are v2 |
| CLI frontend | PLAN.md §8 | v1 ships web-only (M5); the CLI is a v2 frontend |
| Content-key cache store | PLAN.md §2.4, §6.6 | No DRM in v1, so no content-key cache exists |

**Consequence of the DRM deferral:** in v1, download means non-DRM content only. DRM-protected titles stream or download only where the source is already clear; the `drm?` manifest field (§6.2) is carried but never populated by a v1 adapter, and the delivery engine treats a populated `drm?` field as an honest "unsupported in this version" failure — never a silent skip.

## 2. What "v1 done" means

v1 is done when all of M0–M6 pass their acceptance criteria (IMPLEMENTATION.md §3), all of which are enforced by the Definition of Done (IMPLEMENTATION.md §6) and gated in CI (CI-CD.md §5):

- Fixture suites green, including negative fixtures, for every contract and adapter v1 ships.
- Storage classes behave per their PLAN.md §2.4 class — tested, not assumed.
- The vault/secrets suite (TESTING.md §6) is green and never skipped.
- A user can: create an account, link a library-class provider account, see a merged library, browse/search with enrichment, and play or download a non-DRM title — all through the v1 frontend.
- An owner can share an account, and a revoked member's active sessions end mid-stream (M6).
- No secrets in the repository; content-blind logging holds in core and ops paths (§7.6 of PLAN.md).
- The load-bearing contracts are unchanged since M0, or any change is deliberate and recorded in PLAN.md §11.

## 3. Relationship to the other documents

- **IMPLEMENTATION.md** — the milestone roadmap this document cuts; the v1 boundary here names which of its M0–M9 ship first.
- **PLAN.md** — the spec; nothing here changes what the system is, only what ships when.
- **TECHNICAL-DECISIONS.md** — the technology that realizes v1.
- **RESEARCH.md** — feasibility evidence (e.g., the first provider, deferred-DRM rationale) that informs this scope.
- **CI-CD.md** — §5's milestone-tagged releases are gated on the boundary defined here.
