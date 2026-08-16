# CI/CD

This document fixes *how changes to the project are integrated, verified, and shipped*. ENVIRONMENT.md fixes what a machine must contain; TESTING.md fixes what "tested" means; this document fixes the pipeline that makes both of them gate every change. It is platform-neutral by design — the stages are described abstractly, with a platform mapping in §7 (the concrete platform is recorded in TECHNICAL-DECISIONS.md §1.7). Nothing here depends on a specific CI vendor.

The load-bearing idea: **the conformance gate is the quality floor** (IMPLEMENTATION.md §4.2). CI is where "reject, never downgrade" (§2.5 of PLAN.md) becomes mechanical — a slot that fails the suite of the version it declares fails CI, and a milestone whose fixtures do not pass never ships.

## 1. Pipeline model

One pipeline definition per milestone (M0–M9, IMPLEMENTATION.md §3). The pipeline is a linear set of stages; each stage must pass before the next begins, and the whole thing is gated on the check recipe — the identical recipes a developer runs locally (ENVIRONMENT.md §2, §5; recipe names in TECHNICAL-DECISIONS.md §1.6). CI never invents its own commands.

| Stage | What it runs | Gate it enforces |
|---|---|---|
| **lint** | Formatting and style checks on all code and schemas | Style is non-negotiable and mechanical |
| **typecheck / build** | Compile the core and adapters; regenerate and verify schema-generated code is up to date | The repo compiles from a clean checkout |
| **schema checks** | Lint and breaking-change detection on schemas (§3) | Contract drift is caught in CI, not production (§2.5 of PLAN.md) |
| **unit + round-trip** | TESTING.md §4.1, §4.2 | Fast hermetic correctness |
| **fixtures** | The fixture suite for every built-in adapter, at the version each declares (TESTING.md §3) | The conformance gate — the load-bearing stage |
| **integration** | Vertical-slice tests (TESTING.md §4.3), including the vault/secrets suite (TESTING.md §6) | Cross-layer wiring works end to end |
| **image** | Build the container image from the same toolchain the tests ran in | The shippable artifact is reproducible (§5) |

Stages are grouped so that failures are cheap and fast: lint/typecheck/schema checks fail in seconds; fixtures and integration are the slow, decisive stages. A PR that fails lint should not spend minutes in the fixture stage.

## 2. The conformance gate in CI

This is the stage CI exists for. On every change, **every built-in adapter runs the fixture suite of the exact version it declares** (IMPLEMENTATION.md §4.2, TESTING.md §3). Rules:

- A failed claim is a rejection — the build fails, the adapter is not admitted, and the reason is surfaced in the registry's output.
- Negative fixtures are required for anything that accepts input; vacuous permissiveness is a test failure.
- The DRM slot's composition fixtures (synthetic encrypted media, known keys — §6.6 of PLAN.md) run in this stage: they prove the acquire-keys → decrypt → compose pipeline without any live license server.
- The vault/secrets suite (TESTING.md §6) always runs; it is never skipped for speed.

No commit is merged and no image is built past a fixture failure.

## 3. Schema checks

Contract drift is the biggest build-time risk (IMPLEMENTATION.md §7), so the schemas get their own stage:

- **Lint** the schema files for style and structural consistency.
- **Breaking-change detection** on every change to a schema: an additive change (§3.4 of PLAN.md — new fields) is fine and does not bump a version; anything else requires a new contract version and a new handshake. The check fails if a committed change bumps nothing but breaks the wire.
- **Generated-code freshness**: the pipeline regenerates code from schemas and fails if the committed generated code is stale. "It compiles" is not enough; it must compile from the checked-in state.

Because the load-bearing contracts are frozen once approved, changes to them are the most scrutinized in the pipeline: a change to any of them requires the explicit recorded decision in PLAN.md's decision log (§11 of PLAN.md) before the stage can pass.

## 4. Secrets in CI

The same secrets discipline that governs the codebase governs the pipeline (ENVIRONMENT.md §6):

- **No secrets in the repository.** Test credentials come from CI secret storage, injected into the job environment, never committed, never in logs.
- **Vault/decrypted material never appears in logs or artifacts** (IMPLEMENTATION.md §1.3). The pipeline treats a leaked secret in an artifact or a log line as a stage failure, not a warning.
- **Sink and DRM slots are tested with mock fixtures only** (TESTING.md §7); CI never holds a real credential, and the manual DRM verification path is documented, never automated with live content.
- A `.env.example` documents the *names* of configuration variables; values never enter the repository (ENVIRONMENT.md §6).

## 5. Delivery / releases

CD is deliberately modest — the artifact is the container image (§1, image stage), built from the same pinned toolchain the tests ran in (ENVIRONMENT.md §1), so what is tested is what ships.

- **Green main ships; nothing else does.** Images are built and published only from a main that passed the full pipeline.
- **Milestone-tagged releases.** Releasing a milestone (M0–M9) means: fixtures green in CI, the milestone's acceptance criteria green, the load-bearing contracts unchanged or the change recorded in PLAN.md's decision log, and — if the milestone surfaced a product decision — that decision recorded (§6 of IMPLEMENTATION.md).
- **Reject, never downgrade, applies to releases too.** A failed stage means no artifact; there is no "ship it anyway" path.
- **Rollbacks are trivially available** because images are immutable and tagged by milestone; reverting is redeploying a known-good tag.

## 6. Branch and PR policy

- **Short-lived branches; the PR is the unit of review.** Every PR runs the full pipeline; the pipeline result is part of the review.
- **Protected main.** Merges require the pipeline green. Nothing bypasses the conformance gate.
- **Milestone completion is a pipeline event.** A milestone is done when its fixtures pass and CI is green (IMPLEMENTATION.md §3) — the pipeline is the enforcement, not the hope.
- **Local == CI.** Because CI runs the exact recipes from ENVIRONMENT.md, a developer's green check recipe predicts a green CI run, and a CI-only failure is an environment bug, not a developer problem.

## 7. Platform mapping

The stages in §1 are vendor-neutral. The concrete platform and its exact mapping are recorded in TECHNICAL-DECISIONS.md §1.7; the general shape transfers to any platform with the same stages:

- **lint, typecheck/build** — a job per stage on a fresh runner from the pinned toolchain (the same image as ENVIRONMENT.md §3).
- **schema checks** — a dedicated job running the schema lint and breaking-change tooling against the schemas; fail on breaking change without a version bump.
- **unit + round-trip** — a job running the fast hermetic suites.
- **fixtures** — the load-bearing job: runs the fixture suites for every built-in adapter. This job is the conformance gate and is a required check on every PR.
- **integration** — the cross-layer job, including the vault/secrets suite (never skippable).
- **image** — built only when the prior stages pass; tagged by milestone on green main.

The general shape — required PR checks, protected main, image-on-green-main — transfers to any other platform with the same stages. The recipes and the suite are the portable part; the platform is interchangeable.

## 8. Performance and caching

- **Dependencies and generated code are cached** across runs (package caches, codegen output), keyed by the version pins — a change to a pin invalidates the cache, so caching never hides a real change.
- **Cheap stages fail fast.** Lint/typecheck/schema checks run before fixtures and integration, so a formatting mistake never spends minutes in the conformance gate.
- **The fixture stage stays parallelizable** — each adapter's suite is independent — so adding an adapter adds a parallel slice, not serial wall-time.
- **The vault/secrets suite is not cached or skipped.** It is the one stage whose cost is non-negotiable.

## 9. Operational health of CI itself

The pipeline is infrastructure and gets the same care as the product:

- **Flaky-test policy.** A test that intermittently fails is a bug; it is quarantined, fixed, and returned — never suppressed. A green pipeline must mean green, not "green this time."
- **The fixture suites are deterministic** (TESTING.md §8): fixed seeds, no randomized behavior, so a pass is reproducible.
- **Self-hosted note.** Integration tests that spawn subprocess slots (transport tests, supervision tests — §2.1, §4 of PLAN.md) need a runner that permits process-spawning; if the hosted runner forbids it, a self-hosted runner with the pinned toolchain is the documented alternative. The stages and gates are identical either way.

## 10. Relationship to the other documents

- **ENVIRONMENT.md** — CI runs its recipes verbatim (§2, §5); the container CI builds from is the container developers use (§3).
- **TESTING.md** — CI executes its levels as gates (§2); the conformance gate here is TESTING.md §3 made mechanical.
- **IMPLEMENTATION.md §6** — the Definition of Done this document enforces: "CI green" is this document's whole reason to exist; "built-in adapters run their suites in CI" is §2.
- **TECHNICAL-DECISIONS.md** — records the CI platform and version pins this pipeline uses. **SCOPE.md** — defines which milestones gate a v1 release (§5).