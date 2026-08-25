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

### 2.1.1 M1 live spike — **done** (verified against a Jellyfin 10.11 instance)

Evidence behind TECHNICAL-DECISIONS.md §1.8 and §1.21; **not** a decision. Verified with a
throwaway probe script plus the real adapter driving a live server over `AuthenticateByName`
and `/Items`.

- **Auth flow:** `POST /Users/AuthenticateByName` with JSON `{Username, Pw}` and a
  `MediaBrowser`-scheme `Authorization` header returns `{AccessToken, User.Id}` in the body.
  The legacy `X-Emby-Token` header is deprecated and removed on 10.9+ — tokens travel only
  in the header scheme. Wrong credentials are rejected cleanly (HTTP 401).
- **Catalogue index:** `GET /Items` with `userId`, `includeItemTypes=Movie,Series`,
  `recursive=true`, offset params (`startIndex`/`limit`) answers with
  `{Items[], TotalRecordCount, StartIndex}`. Offset pagination is sufficient behind opaque
  continuation tokens.
- **Identity metadata:** `BaseItemDto.ProviderIds` carries exactly the §5.3 assertion shape,
  e.g. `{"Imdb": "tt1424797", "Tmdb": "26280"}`. **On 10.9+ it is omitted unless explicitly
  requested** via `fields=ProviderIds` — verified by A/B query; without it every identity
  assertion silently disappears from the index. The adapter pins this with a fixture-backed
  regression test.
- **Item-ID stability:** native IDs are stable across repeated fetches of an unchanged
  library (verified by diffing consecutive page fetches). A rescan-rescan stability check
  remains outstanding until the test library changes, but no mechanism suggests instability:
  IDs are content-addressed GUIDs, not list positions.
- **Rate limits / detection posture:** no documented rate limiting anywhere in the spec or
  observed behavior; defensive backoff is kept anyway (TECHNICAL-DECISIONS.md §1.22).
- **Not yet probed (deferred to their own milestones):** playback/session endpoints for
  `produce-sources` manifests (§6.2), and multi-user scoping beyond a single account.

### 2.2 DRM baseline (pre-spike notes)

Recorded as the starting point for the v2 spike; **not** a decision.

- License negotiation is a challenge/response exchange (init data → license server → keys), which maps to `acquire-keys` (§6.6) at the message-shape level.
- Whether a real adapter is feasible depends on: a specific provider, its CDM/lockdown posture, and device-credential lifecycle. The plan's mock/synthetic-fixture admission path (TESTING.md §7) is designed exactly for this uncertainty.

### 2.3 Embedding-surface layout survey (M0)

Evidence behind the `core/app` bootstrap seam recorded in TECHNICAL-DECISIONS.md §1.2; **not** a decision.

- Go's `internal/` rule is directory-based: `core/internal/*` is importable only from within `core/`, so any in-process embedder outside `core/` needs an exported package that itself imports the internals — the wrapper must sit inside the boundary's parent directory for the import to be legal.
- **etcd** ships exactly this shape as its public embedding API: `go.etcd.io/etcd/server/v3/embed` (`StartEtcd(cfg) (*Etcd, error)`, `Close()`, plus hooks like `ServiceRegister func(*grpc.Server)`) lives inside the server module and imports server internals; consumers never see them.
- **HashiCorp Vault** is the stronger form: the exposed contract lives in its own separately versioned module (`hashicorp/vault/sdk`, alongside `api`), so plugin authors physically cannot reach implementation internals — at the cost of multi-module sync and release discipline.
- **Kubernetes** publishes curated `k8s.io/*` staging modules (client-go etc.) while its monorepo `pkg/` tree stays de-facto internal by convention.
- **CockroachDB** puts everything under `pkg/` with almost no `internal/`; layering rules are enforced by custom lint tooling rather than the compiler.
- The `golang-standards/project-layout` README itself notes `pkg/` is convention only and "`internal` ... is enforced by Go" — the compiler mechanism is the stronger gate.
- Consequence observed in-repo: with consumers inside the same module (frontends), a root-level `pkg/` either cannot import `core/internal/*` at all, or (after moving internals to a root `internal/`) would let frontends bypass the seam entirely. Drawing the boundary at `core/` with the facade beside the internals gives compiler enforcement of the narrow surface.

### 2.4 TMDB API baseline (desk spike)

Evidence behind TECHNICAL-DECISIONS.md §1.27; **not** a decision. Desk research against TMDB's
published developer docs (no live calls); live verification rides with the adapter's fixture work.

- **Auth:** v3 API key or v4 API Read Access Token as `Authorization: Bearer …`. Bearer mode
  preferred; free tier, no account-link flow (unlike provider slots).
- **Lookup:** `/3/search/movie` and `/3/search/tv` (`query` required; optional `year` /
  `first_air_date_year` hints) answer `{page, results[], total_pages, total_results}`. Result rows
  are summary-shaped: `id`, `title` (movies) or `name` (series), `release_date` /
  `first_air_date`, `genre_ids[]`, `poster_path`.
- **Identity bridge:** `/3/find/{external_id}?external_source=imdb_id|tvdb_id|wikidata_id|…`
  answers `movie_results[]`/`tv_results[]` with the same summary shape plus TMDB `id`. IMDb
  resolves both movies and series; TVDB resolves series only.
- **Details:** `/3/movie/{id}` and `/3/tv/{id}` accept `append_to_response` (≤20 sub-endpoints) so
  details + `external_ids` (`imdb_id`, `wikidata_id`) + `credits` (cast, crew→directors; series
  creators via `created_by`) + content ratings arrive in one request. Movies carry `runtime`;
  series carry episode runtimes and season counts.
- **Images:** poster/backdrop paths are relative; full URL =
  `https://image.tmdb.org/t/p/<size><path>` (e.g. `w500`). Canonical base URL comes from
  `/configuration`.
- **Rate limits:** soft ~40 req/s; exceeding returns HTTP 429 with `Retry-After` which clients
  must honor.
- **Attribution (contractual):** "This product uses the TMDB API but is not endorsed or certified
  by TMDB." plus logo rules — a frontend obligation (M5), recorded here because accepting the
  API's terms happens at adoption.

## 3. How findings become decisions

1. A spike completes and its findings are written here.
2. If the finding changes the product, PLAN.md is edited (and its §11 log updated) — per IMPLEMENTATION.md §3.
3. If the finding changes scope, SCOPE.md is updated and re-signed.
4. If the finding only confirms the plan, it is noted here and no decision doc changes.

## 4. Relationship to the other documents

- **TECHNICAL-DECISIONS.md** — the decisions this evidence supports (e.g., §1.8's first-provider choice is confirmed by spike 1).
- **SCOPE.md** — scope changes driven by findings.
- **PLAN.md** — the spec; any finding that contradicts it is a PLAN.md change first.
