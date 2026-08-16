# Research and Spike Findings

This document holds the **evidence** behind feasibility decisions. TECHNICAL-DECISIONS.md records *what was decided*; this document records *what was learned* and why. Findings here inform scope (SCOPE.md) and decisions, but a finding is not itself a decision — a decision is recorded in TECHNICAL-DECISIONS.md or SCOPE.md, never only here.

Each spike has a status: `pending` → `in progress` → `done`. Nothing here is a commitment until a decision doc records it.

## 1. Planned spikes

| Spike | Questions it answers | Gates | Status |
| --- | --- | --- | --- |
| **First provider feasibility** | Does Jellyfin's API cleanly map to whole-catalogue sync (§5.4)? Pagination shape? Item metadata + external IDs? Rate-limit posture? | M1 start | pending |
| **Relay-through-owner feasibility** | Double-hop latency, owner bandwidth cost, connection model (§7.4). Is it workable for a v2 sidecar? | v2 planning | pending |
| **DRM slot feasibility** | Which DRM systems (Widevine L1/L3, FairPlay, PlayReady) map to `acquire-keys` + `decrypt` (§6.6)? What real license-negotiation shapes exist? Is decrypt-to-clear achievable, and for which systems? | v2 planning (DRM is out of v1 per SCOPE.md) | pending |

## 2. Findings

### 2.1 Jellyfin API baseline (pre-spike notes)

Recorded as the starting point for the M1 spike; **not** a decision.

- REST API with an `/Items` index (whole-library query), stable media item model, and external IDs (IMDb/TMDB) in item metadata where present.
- Library query is paginated (`StartIndex`/`Limit`) — matches §5.4's "pagination, not entry count" cost model.
- Authentication via API keys or user token — a clean fit for the *link-account* axis (OAuth/API token mechanism, §3.5).
- Open-source server (AGPL-licensed), no detection-risk posture comparable to commercial streaming services.
- To be verified during the spike: exact endpoint shapes, item-ID stability across refreshes, metadata completeness (title/year/external IDs) for the identity registry (§5.3), and what a *media source* manifest (§6.2) looks like for `produce-sources`.

### 2.2 DRM baseline (pre-spike notes)

Recorded as the starting point for the v2 spike; **not** a decision.

- License negotiation is a challenge/response exchange (init data → license server → keys), which maps to `acquire-keys` (§6.6) at the message-shape level.
- Whether a real adapter is feasible depends on: a specific provider, its CDM/lockdown posture, and device-credential lifecycle. The plan's mock/synthetic-fixture admission path (TESTING.md §7) is designed exactly for this uncertainty.

## 3. How findings become decisions

1. A spike completes and its findings are written here.
2. If the finding changes the product, PLAN.md is edited (and its §11 log updated) — per IMPLEMENTATION.md §3.
3. If the finding changes scope, SCOPE.md is updated and re-signed.
4. If the finding only confirms the plan, it is noted here and no decision doc changes.

## 4. Relationship to the other documents

- **TECHNICAL-DECISIONS.md** — the decisions this evidence supports (e.g., §1.8's first-provider choice is confirmed by spike 1).
- **SCOPE.md** — scope changes driven by findings.
- **PLAN.md** — the spec; any finding that contradicts it is a PLAN.md change first.
