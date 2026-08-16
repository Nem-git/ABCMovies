# Implementation Plan

This document is the bridge between the spec (PLAN.md) and a codebase that can grow. PLAN.md fixes *what* the system is and *why* the decisions were made; this document fixes *how it gets built and how it is kept buildable*. It is a process and sequencing document, not a second spec. Where this document and PLAN.md disagree, PLAN.md wins — a change to PLAN.md is a change to the product and is made by editing PLAN.md itself, not this document.

The single most important idea: **PLAN.md is a spec, not a backlog.** It is not a list of tasks to grind through in section order. Requirements must be made *testable* (§1.1), and the build must be sequenced so the riskiest, load-bearing parts are proven first with the thinnest possible end-to-end path (§2, §3), then grown capability by capability (§4).

## 1. Making the plan buildable

### 1.1 Testable requirements

A requirement in PLAN.md is only useful to a builder if it can be checked. Two rules make this work:

- **Requirement keywords (MUST / SHOULD / MAY).** Wherever PLAN.md states a rule, the implementation paraphrases it with a keyword so it can be turned into a test or a fixture. Example: "a slot *must* pass the fixture suite of the exact version it declares" (§2.5) becomes a CI check on every adapter; "enrichment *may* inform identity, though it never requires it" (§5.2) becomes a documented, untested-by-default behavior. The keywords are the contract between the plan and the test suite.
- **One requirement → one fixture.** The fixture suite (§2.5 of PLAN.md) is not just a conformance gate for slots; it is also the mechanism for testing the core. Every load-bearing contract's fixtures are written *before* the implementation that satisfies them (§3). A requirement without a fixture is not done.

### 1.2 The load-bearing set is the foundation

PLAN.md §2.3 defines the load-bearing contracts — the canonical set, not re-enumerated here. Everything else bolts onto them. Concretely this means:

- Their schemas are the first schema files written, and they are treated as **frozen once approved**. Additive changes per §3.4 of PLAN.md are fine; breaking changes to this set are the most expensive changes in the system and should be avoided unless a milestone proves them necessary.
- They get the first fixture suites, the first fuzz/round-trip tests, and the most careful review. Cost of a late change here is the highest in the system.
- Nothing downstream is designed against a load-bearing object until that object's fixtures pass. This is what keeps the build linear instead of speculative.

### 1.3 The two trust classes carry implementation duties

PLAN.md §7.6 splits the system into encrypted user blobs and policy-not-proof vault material — the distinction is trust and disclosure, not mathematics. This is not just a privacy story; it changes *what code may exist*:

- Key derivation is **client-side**; the server must never see the raw password or recovery key. This puts a hard boundary in the API: login submits a derived material — which the server then holds as the unwrapping key — never a plaintext secret.
- The vault is decrypted in memory only during a relay, and the host-held relay key (§7.6 of PLAN.md) is a disclosed server secret, scoped to account sessions and never to user blobs. These are operational disciplines the core code must enforce (no persistence, no logging of decrypted material), not features that can be bolted on later. Build the discipline into the vault's test suite from day one.

## 2. Build order: prove the skeleton first

Do not build features in PLAN.md's section order. Build a **walking skeleton** — the thinnest vertical slice that passes through every layer — and then add capabilities. The skeleton's job is to prove that the contracts hold end to end before any of the system's width exists.

**Milestone 0 — the walking skeleton.** One in-process slot, the registry, one store, and the API server. The skeleton:

- Boots the registry, handshakes a single built-in slot, passes the meta-contract fixture (§3.3 of PLAN.md).
- Authenticates a user (username + password, client-side key derivation, §7.6).
- Persists one object in each storage class (§2.4): a rebuildable cache, a vault item, a per-user encrypted blob, a job.
- Exposes one synchronous call and one event over the inbound API (§8) to a minimal web client.
- Runs the whole thing under the test suite and in CI.

When M0 is done, the *shape* of the system exists: contracts, storage classes, jobs/events, frontend boundary. Every later milestone is additive.

**Acceptance criteria for M0:**

- Meta-contract and handshake fixtures pass.
- The event bus delivers one job-status event (§9) to the web client.
- The vault test suite proves: encrypted at rest, in-memory-only during use, nothing logged.
- `git log` shows the load-bearing schema files as the first commits, unchanged since.

## 3. Milestone roadmap

Each milestone below is a **capability**: it names what it adds, which PLAN.md sections it realizes, and its acceptance criteria. Milestones are ordered by risk, not by section number — the trickiest logic (matching/merge) and the most load-bearing boundary (MediaSource + delivery) are owned early, when the codebase is small enough to change. Any product decision a milestone surfaces is recorded in PLAN.md's decision log (§11), so the reasoning isn't lost.

| # | Milestone | Realizes (PLAN.md) | Acceptance |
|---|---|---|---|
| M0 | Walking skeleton | §2.1–2.4, §3.3, §8, §9 | §2 above |
| M1 | One library-class provider adapter + account source cache | §4, §5.4 (whole-catalogue sync) | Adapter passes its fixture suite; a real provider's catalogue lands in the account source cache; cache rebuilds from the provider; registry shows capability versions |
| M2 | LibraryEntry + matching + merge + provider item registry | §2.3, §5.1, §5.3 | Fixture suite: external-ID merges, heuristic merges, negative fixtures (no merge without corroboration), recycled-ID handling, merge-conflict events; provider item registry carries proof only, never coverage (§5.3); per-user library derived and cached (§5.1) |
| M3 | Enrichment (catalogue slot) | §2.4, §5.2 | Enrichment off by default; field-level merging across catalogue slots; provenance retained per external ID; heuristic-resolved IDs require corroboration before driving a merge (§5.3) |
| M4 | MediaSource manifest + first delivery session to the built-in sink | §2.3, §6 | Manifest fixtures (§6.2: PER_TRACK, WHOLE_MUX, mixed); one passthrough play session and one remux download session end-to-end; session→account index, heartbeat, TTL, revocation (§7.1, §9.1) |
| M5 | First real frontend on the core API | §8 | Frontend renders search/browse/library from the API; jobs surface as events; one inbound protocol chosen and locked (constraint: the hardest client, §8.1) |
| M6 | Sharing, policy, quotas, audit | §7.1, §7.2, §2.4 | min(policy, provider_cap) enforced at one point; revocation kills live sessions; member-scoping invariant is tested, not asserted (§2.2); availability events are account-scoped and invalidate every affected user's derived cache (§5.1, §9.2) |
| M7 | Streaming-service provider (lazy refresh) + pacing | §5.4, §6.5, §7.2 | Lazy provider never probed in background; usage-confirmed refresh only; shared pacing budget: background work can never exceed user limits; per-provider aggregate governor bounds load across accounts (§5.4); busy answers carry queue position |
| M8 | DRM slot + extra sinks | §6.6, §6.4 | acquire-keys + decrypt composed as decrypt-to-clear on mock fixtures (§2.5 exemption) and on synthetic encrypted fixtures (known keys, §6.6 of PLAN.md); credential set rotates; awaiting-action only at exhaustion; content-key cache is content-keyed and fail-fast (a decrypt failure drops the entry and re-licenses; rotation never purges); a second sink type (e.g. media-server sink) works without core changes |
| M9 | Sidecar custody + egress routing | §3.5, §7.3, §7.4 | relay-through-owner works; egress routing off by default; bulk-direct gated per-provider on adapter-declared CDN tolerance; portable-session remains a documented goal |

**Deferred, deliberately** (recorded in PLAN.md as future or dropped): scoring profiles (§6.3), session coalescing (§6.5), entry-level correction tools (§5.3), license-wrapper composition (§6.6), direct-URL handoff (§6.5), two-factor / identity-provider login (§7.6).

**Release boundary.** This roadmap is the generic reference. Which milestones ship in the first release is defined in SCOPE.md (v1 = M0–M6); the technology that realizes each milestone (core language, API transport, tooling) is recorded in TECHNICAL-DECISIONS.md, not here.

Each milestone ends with: fixtures green, CI green, and — if it surfaced a product decision — that decision in PLAN.md's decision log (§11). A milestone is not done when it compiles; it is done when its fixtures pass and its store classes behave per §2.4 of PLAN.md.

## 4. Growth mechanics

### 4.1 Everything is additive

The design's payoff is that growth is *declared*, not *modified*. A new source is a slot with a fixture suite; a new interface is a frontend against the API. The rules that keep this true:

- **New slots never touch the core.** A slot is added by writing an adapter + fixtures and declaring it in config (§4 of PLAN.md). If adding a slot requires a core change, that is a design violation, not a normal step.
- **New capabilities are per-operation, versioned per operation** (§3.4). Version bumps are the mechanism for changing behavior; the registry records them and the fixture suite gates them.
- **Deletion is a first-class operation.** Removing a capability is a breaking change and gets the same scrutiny as adding one. The graceful-drain rule (§4 of PLAN.md) applies to code too: old versions coexist until sessions drain.

### 4.2 The conformance gate is the quality floor

PLAN.md's "reject, never downgrade" (§2.5) is the enforcement of everything above. In the implementation this becomes:

- **CI runs the fixture suite for every built-in adapter** on every change.
- **No slot is admitted without passing the suite of the exact version it declares** — a failed claim is a rejection, surfaced in the registry, never a silent downgrade.
- **Negative fixtures are required** for anything that accepts input (merge matching, manifest validation, session start). Vacuous permissiveness is a test failure.

### 4.3 Vertical slices over horizontal phases

Within a milestone, work in vertical slices: a request enters the API, crosses the core, touches one store, and returns — then the next request. This keeps every layer exercised continuously and avoids the classic failure of "all the plumbing, none of the features." The walking skeleton (§2) is the first vertical slice.

## 5. Methodologies and tooling to borrow

These are the established practices this plan draws on; each is applied in the specific way noted.

| Practice | Where it applies here |
|---|---|
| **Walking skeleton / tracer bullets** (Pragmatic Programmer) | §2, §3 M0 |
| **Shape Up** (Basecamp) | Decide *what* a milestone is before *how* it's built; no estimates |
| **Domain-Driven Design** | The slot taxonomy and contract schemas are bounded contexts and a shared ubiquitous language; DDD's rules govern how contexts evolve and where seams stay clean (§4.1) |
| **C4 model** | Four diagram levels (context → containers → components → code); core = container, slots/frontends = containers, contracts = component boundaries. Keep the container diagram current per milestone |
| **Contract testing / Specification by Example** | Fixture suites (§1.1, §4.2); "one requirement → one fixture" |
| **Build the boring parts first** | Identity, storage, registry, audit are M0–M2, before any feature richness |
| **Feature flags / strangler pattern** | Adopt providers one at a time behind per-slot config; a broken or experimental slot ships without touching core (§4.1) |
| **Vertical slices** | §4.3 |

## 6. Definition of Done

A milestone (or slice) is done only when all of these hold:

- [ ] Fixture suite for the milestone's contracts passes, including negative fixtures (§4.2).
- [ ] Storage behavior matches its §2.4 class (durability + who reads) — tested, not assumed.
- [ ] CI green; built-in adapters run their suites in CI.
- [ ] The milestone does not require a core change to add a slot or frontend (§4.1). If it did, that was reviewed as a design violation.
- [ ] Secrets discipline: no secrets in the codebase; vault/decrypted material never logged (§1.3, §7.6 of PLAN.md).
- [ ] Any product change surfaced by the work is reflected in PLAN.md — no silent divergence between the spec and the fixtures.
- [ ] The load-bearing contracts are unchanged, or the change was deliberate and recorded in PLAN.md (§11).

## 7. Risk-driven sequencing notes

The milestone order is not arbitrary; it is PLAN.md's own risk model made into an order:

- **Contract drift is the biggest build-time risk** (§2.5, §3.4 of PLAN.md), so fixtures and the conformance gate come first (§1.1, §4.2).
- **Matching/merge is the trickiest logic** (§5.3 of PLAN.md), so it is owned at M2 while the codebase is small.
- **MediaSource + delivery is the load-bearing boundary between content and everything else** (§6.2 of PLAN.md), so it is proven at M4 before frontends or sharing are built on top.
- **Lawful-sensitive work (DRM) and account-shape work (sharing) are deferred until the core is proven**, because they compound the core's risk rather than reduce it.
- **Provider-side detection risk** (§5.4, §7.2, §7.3 of PLAN.md) is inherent to the domain: pacing, lazy refresh, and per-provider cadence are designed in from M1/M7, not retrofitted.

The sequence is a recommendation, not a straitjacket. Reorder only with a written reason; the reason goes into PLAN.md's decision log (§11).

## 8. Keeping dependencies current

Dependency updates are a routine maintenance flow, not a milestone feature. They follow the same discipline as everything else: the environment is *declared*, not *accumulated* (ENVIRONMENT.md §2), so an update is never an ad-hoc host install — it is a change to the declared pins, verified through the same gate as any code change.

### 8.1 Three tiers, three rules

| Tier | What it is | Where it lives | Update rule |
|---|---|---|---|
| **Toolchain pins** | The schema compiler, schema-language plugins, the core runtime | The canonical pin file (TECHNICAL-DECISIONS.md §1.4) | Bump the pin; the change invalidates CI caches (CI-CD.md §8) and triggers an image rebuild (ENVIRONMENT.md §3) |
| **Language dependencies** | Core and adapter libraries, installed by the dependency recipe | Language manifest, resolved from the pinned versions | Ordinary bump-and-verify through the check recipe |
| **Contract versions** | The schemas — frozen once approved (§3.4 of PLAN.md) | The schema files | Never modified without explicit approval. A change is not a bump: it needs a decision-log entry (§11 of PLAN.md), a new fixture suite (TESTING.md §3), and a breaking change requires a new version and a new handshake (§3.4 of PLAN.md) |

### 8.2 The per-update workflow

Every update — whether opened by the automation bot or done by hand — follows the same steps:

1. **Edit the single source of truth**: the canonical pin file (TECHNICAL-DECISIONS.md §1.4) for toolchain pins, or the language manifest for library deps. Nothing is installed ad hoc on a host (§2 of ENVIRONMENT.md).
2. **Rebuild the container image** from the updated image definition (TECHNICAL-DECISIONS.md §1.3); the new image is what both dev and CI run, so drift is impossible (ENVIRONMENT.md §3).
3. **Run the dependency recipe**, then the **schema-codegen recipe** (`make proto`, TECHNICAL-DECISIONS.md §1.6) if the change affects codegen.
4. **Run the check recipe locally** — the identical CI gate. A green local run predicts a green CI run by construction.
5. **Open the PR**; the full pipeline runs (lint → build → schema checks → unit → fixtures → integration → image, CI-CD.md §1). Caches are invalidated automatically because they are keyed by the pins (CI-CD.md §8).

**Reject, never downgrade applies to dependencies too** (§2.5 of PLAN.md): an update that fails its fixture suite is rejected — the pinned version stays, the update does not pass by pinning back or "working around" the failure.

### 8.3 Security-sensitive dependencies

Updates touching key derivation, transport encryption, or vault/encryption material get extra scrutiny: the PR is flagged for human review, and the vault/secrets suite always runs and is never skipped (CI-CD.md §2, TESTING.md §6). A "just a bump" is never sufficient for these; the change must land with the secrets suite green.

### 8.4 First activation

The workflow goes live at M0 scaffolding, when the initial pins are frozen (TECHNICAL-DECISIONS.md §1.4) and the automation config lands (TECHNICAL-DECISIONS.md §1.9). Until then the process is documented, not yet executed.