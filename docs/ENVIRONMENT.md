# Environment Setup

This document is the bridge between a fresh machine and a working checkout of the project. PLAN.md fixes *what* the system is; IMPLEMENTATION.md fixes *how it is built and in what order*; this document fixes *what a developer's machine must contain and how a local instance comes up*. It is deliberately language-agnostic: the contracts are schemas, and any slot or frontend may be written in any language (§2.1 of PLAN.md). Where a tool appears, it is the tool the fixture runner or the test suite requires, not a language mandate.

Nothing here is sacred except reproducibility: the environment that produces a green CI run must be reproducible on a developer machine, and the environment a developer uses must be reproducible in CI. The rule that keeps this honest is **local == CI**: the commands in §5 are exactly the commands CI runs (§4 of CI-CD.md), no more and no less.

## 1. Prerequisites

The bare minimum for a checkout to build and run the test suite:

- **Schema compiler** — the compiler for the contract schema format (chosen and recorded in TECHNICAL-DECISIONS.md §1.10), pinned to a specific version (recorded in the toolchain pin or equivalent). The schemas are the single source of truth for contracts (§2.1 of PLAN.md); nothing compiles without it.
- **Schema-language plugins** — one per language actually used in the repo (TECHNICAL-DECISIONS.md §1.10). Plugins are pinned to versions; unpinned plugins are a reproducibility failure.
- **A runtime for the core** — whatever language the core is written in (TECHNICAL-DECISIONS.md §1.1), plus that language's package manager and linter. Chosen at implementation time (IMPLEMENTATION.md §2, M0); until then, only the schema compiler and the plugin set are required.
- **A task runner** — the single entry point that wires everything: codegen, build, and test (the chosen runner and its recipes are recorded in TECHNICAL-DECISIONS.md §1.6). If it is unavailable on a platform, the documented task-runner equivalent must run the *same* recipes.
- **A container tool** — any conformant one, optional but recommended (§3). Nothing in this document depends on which. If the host has none, use §4 (plain host setup).
- **A version-control tool** — the repository itself.

Version pins live in a single canonical location (TECHNICAL-DECISIONS.md §1.4), referenced by the env doc, the CI config, and the dev container. There is exactly one source of truth for "what versions do we build with", and CI and developers both read it from there.

## 2. The one rule: local == CI

Every development environment, and every CI job, runs from the same recipe. Concretely:

- The same pinned versions (§1) are used locally and in CI.
- The same build/test entry point (the task-runner recipes, TECHNICAL-DECISIONS.md §1.6) is used locally and in CI — CI does not invent its own commands (§5 of CI-CD.md).
- The environment is *declared*, not *accumulated*: a developer never fixes a broken build by installing something ad hoc on their host, because that fix would not exist in CI and would therefore not be reproducible. The fix belongs in the recipe.

If a step is not written down here, it is not part of the environment.

## 3. Option A — reproducible container (recommended)

A single container-image definition (TECHNICAL-DECISIONS.md §1.3) declares the full toolchain: the schema compiler, the pinned plugins, the core runtime, the linter, and the fixture-runner CLI. The same image serves two purposes:

- **Dev container** — an editor opens the checkout *inside* the container, mounting the workspace, so every developer sees the identical toolchain. The container is not a runtime per se; it is the *place where development happens*. Any conformant container runtime works.
- **One-off commands** — run the build, test, etc., recipes against the image, without any host-side toolchain.

Both paths use the identical image, so there is no drift between "developing in a container" and "running commands in a container." The container is the canonical environment; a developer who never uses it is on their own (§4).

The image is rebuilt on any version-pin change and is what CI's build stage is based on (CI-CD.md §3), so CI and developers run the same image by construction.

## 4. Option B — plain host setup

For hosts without a container tool, the same environment is assembled manually. This path is best-effort: it can drift, because it depends on the host. The steps are exactly what the container-image definition does, done by hand:

1. Install the schema compiler at the pinned version (§1).
2. Install the pinned schema-language plugins.
3. Install the core runtime and its package manager.
4. Install the task runner.
5. Run the dependency recipe (installs the language-level dependencies) and the schema-codegen recipe — `make proto` (regenerates code from schemas; recipe names in TECHNICAL-DECISIONS.md §1.6).
6. Verify with the check recipe — the recipe that lints, builds, and runs the full suite locally (§5).

If any step does not match the container path, the container wins: the container-image definition is the source of truth.

## 5. Bringing up a local instance (M0)

The walking skeleton (IMPLEMENTATION.md §2, M0) is the smallest thing that proves the environment works: it boots the registry, handshakes a single in-process slot, authenticates a user, persists one object per storage class, and exposes one synchronous call and one event to a web client. The environment is correct when the M0 acceptance criteria pass:

- The check recipe is green (lint, build, fixture suites, tests).
- The registry boots and handshakes the built-in slot; the meta-contract fixture passes (§3.3 of PLAN.md).
- A user can authenticate (username + password, client-side key derivation — §7.6 of PLAN.md, IMPLEMENTATION.md §1.3).
- One object persists in each storage class (§2.4 of PLAN.md): a rebuildable cache, a vault item, a per-user encrypted blob, a job.
- One synchronous call and one event reach the web client.

The concrete sequence (recipe names per TECHNICAL-DECISIONS.md §1.6):

```shell
deps        # language-level dependencies, from the pinned versions
proto       # regenerate code from the schema files
check       # lint + build + full suite + vuln — the CI gate, run locally
run         # boot the skeleton: registry, slot, API server
```

`make lint` inside the check recipe also runs the formatting and hygiene checks — gofumpt, prettier, markdownlint, and the secret-leak scan (recipe table and pin set in TECHNICAL-DECISIONS.md §1.4, §1.6).

The check recipe and the CI pipeline run the identical recipes; a green local run is the developer's personal CI run.

## 6. Config and secrets for a local instance

A local instance reads its configuration from a single YAML file (default location is recorded in TECHNICAL-DECISIONS.md §1.5, not here — this document stays language- and layout-agnostic; the file is gitignored, with a committed `.example` template). The layout mirrors PLAN.md §2.4:

- **Caches** — source cache, enrichment cache, content-key cache, derived library cache. Rebuildable, safe to lose. For dev, an in-memory or lightweight embedded default is fine (IMPLEMENTATION.md §2, M0 persists one object per class to prove the class exists, not to provision real stores).
- **Vault** — durable, must not lose. Account sessions encrypted at rest under per-session keys, wrapped by the owner's KEKs and the host relay key (§7.6 of PLAN.md). In dev this is a local file; the discipline (in-memory-only during use, never logged, relay key scoped to sessions and never to user blobs — IMPLEMENTATION.md §1.3) applies identically in dev and production. A dev vault must never contain a real credential.
- **Watch history / playlists** — durable, per-user encrypted. In dev, a lightweight embedded store or in-memory.
- **Job/session state** — checkpointed. In dev, in-memory.
- **Event bus** — ephemeral. In dev, in-process.

**Secrets policy (non-negotiable, mirrors CI-CD.md §4):** no secrets in the repository. A `.env` file (gitignored) supplies local test credentials; a `.env.example` documents the *names* of the variables, never their values. The vault's encryption keys are generated locally, never committed, never logged.

## 7. Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| The schema-codegen recipe fails | The schema compiler or a plugin at the wrong version | Recheck against the pin (§1); rebuild the container (§3) |
| Generated code is stale | Schemas changed, codegen not rerun | Run the schema-codegen recipe (`make proto`); CI would have caught this (CI-CD.md §3) |
| "unknown field" errors in fixtures | Consumer at wrong contract version | Version per §3.4 of PLAN.md; rerun the check recipe |
| Port collision on the `run` recipe | Another instance or service on the port | Change the port in the local config; never the default |
| In-process slot crashes the core | Slot is untrusted / at fault | Per §4 of PLAN.md, only trusted first-party code runs in-process; run the slot subprocess instead |
| Tests pass locally, fail in CI | Environment drift | Local == CI is violated; fix the recipe, not the host (§2) |

## 8. Relationship to the other documents

- **TESTING.md** — defines *what* the suite contains (fixture suites, unit, integration). This document's check recipe is the mechanical entry point that runs it.
- **CI-CD.md** — defines *where* the suite runs and how it gates releases. It runs the same recipes this document defines; it never invents its own commands.
- **IMPLEMENTATION.md §6** — the Definition of Done that all three documents serve. "CI green" is unreachable without this document's reproducibility, and this document's reproducibility is only meaningful because TESTING.md defines what green means.
- **TECHNICAL-DECISIONS.md** — records the implementation choices (core runtime, version pins, config location, repo layout) this document deliberately stays agnostic about.
