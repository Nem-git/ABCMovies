# AGENTS.md

This file is the contract for AI agents working in this repository. It governs
how an agent develops the codebase and how it interacts with the human user.
Read it fully before doing anything.

## 1. Orientation: the docs are the source of truth

- This project is **ABCMovies**, a self-hosted media hub. It is *specified
  before it is built*: the documents in `docs/` are the reference, and the
  codebase implements them. There is no "obviously right" design that is not
  written down.
- Before doing any work, read `docs/PLAN.md` §1 and `docs/SCOPE.md`, then the
  rest of `docs/`. You are expected to know what every document contains, so
  that when you don't know something you can go look it up instead of
  guessing.
- Document precedence (fixed by the documents themselves):
  - `PLAN.md` is the spec and wins over every other document
    (`IMPLEMENTATION.md` §1).
  - `RESEARCH.md` findings are *evidence, not decisions* — a finding only
    becomes a decision when a decision document records it.
  - Decisions are routed: product decisions → `PLAN.md` §11; implementation
    choices → `TECHNICAL-DECISIONS.md`; scope/acceptance → `SCOPE.md`;
    research evidence → `RESEARCH.md`; threats → `THREAT-MODEL.md`.
- The **load-bearing contracts** (`PLAN.md` §2.3) are the most carefully
  guarded parts of the system. Their schemas are frozen once approved.

## 2. Ask, don't assume

- Never assume the developer's wants or how the project should be designed.
  Ask questions — as many as you need — and give examples with simple
  explanations wherever possible.
- When you don't know something, check `docs/` first. If the docs don't
  answer it, ask the user.
- Treat the open items in `TECHNICAL-DECISIONS.md` §2 as questions for the
  user, never as decisions you get to make unilaterally.
- **Explain, don't cite.** When you tell the user what a document says,
  paraphrase the substance — never quote internal anchors like §2.4 or M4
  and expect the user to know them. Anchors exist for navigating the docs,
  not for the user to memorize; the user should never need to remember
  where anything lives.
- **Push back.** Offer reasoned pushback on ideas — the user's and your
  own — with tradeoffs and alternatives, so the best solution is reached
  by discussion. Agreement without scrutiny is not helpful.

## 3. The plan stays a plan; decisions have one home

- **PLAN.md is a plan, not an implementation doc.** It fixes *what* the
  system is and *why*. Exact numbers and concrete choices — timeouts, TTLs,
  cadences, crypto parameters, version pins, languages, protocols, formats,
  tools — do not belong in it. Where a spec point needs a value, PLAN.md
  states the required behavior and points to the document that owns the
  number.
- **Every value and choice lives exactly once.** Repetition makes a value
  hard to change: once a number exists in several places, there is no
  single source of truth to update. Homes: implementation choices and
  concrete values → `TECHNICAL-DECISIONS.md`; machine pins →
  `.tool-versions`; operational guidance → `OPERATIONS.md`; schema-level
  values → the contract schema (`.proto`). Before writing a number or a
  technology anywhere, search the docs for where it already lives — or
  should live — and reference that instead.
- **Never repeat a value "for convenience."** Introducing a repetition is a
  docs problem: propose the fix via the protected-file process in §4, don't
  add it silently.
- **Decision-log entries record decisions, not values.** If a PLAN.md §11
  entry would have to restate a number to make sense, the number lives in
  the decision document and the entry points there.

## 4. Protected files: never edited without explicit approval

The following may not be edited without the user's explicit approval, given
after you state *what* you want to change, *why*, and provide before/after
examples:

- `proto/` — the contract schemas (`.proto` files). Additive changes (new
  fields) do not bump a version; breaking changes require a new contract
  version and a new handshake (`PLAN.md` §3.4).
- `docs/` — every reference document.
- Version pins and build definitions: `.tool-versions`, `Containerfile`,
  `Makefile`, CI config (`.github/`), `renovate.json`.
- Config and secrets templates: `config.example`, `.env.example`.
- `AGENTS.md` and `opencode.json`.

For `proto/` changes, also state the conformance consequences: which fixture
suites change, whether the change is additive or breaking, and — for anything
breaking — that it needs a `PLAN.md` §11 decision-log entry. "Explicit
approval" means the user has said yes to the concrete proposal, not a vague
"go ahead".

## 5. The code must match the docs

- Always verify that what the code does matches what the docs say. If it
  doesn't, that is a problem to resolve with the user — never resolved
  silently by picking a side.
- The two ways to resolve a mismatch:
  1. Change the docs — following the protected-file process in §4; or
  2. Change the code — and check with the user that the code actually
     reflects what was wanted.
- If the docs don't have an answer for a question, ask the user, then record
  the answer in the appropriate document so the change is wanted and becomes
  part of the project.

## 6. Testing and verification

- **A requirement without a fixture is not done** (`IMPLEMENTATION.md` §1.1).
  Follow `docs/TESTING.md`: fixture-first, negative fixtures mandatory for
  anything that accepts input, round-trip/fuzz on the load-bearing contracts,
  storage-class tests per `PLAN.md` §2.4, and the vault/secrets suite.
- One requirement → one fixture; keep requirement keywords
  (MUST / SHOULD / MAY) traceable to fixtures.
- **local == CI** (`ENVIRONMENT.md` §2). Use the Makefile recipes
  (`make deps`, `make proto`, `make check`, `make run`); never invent your own
  commands.
- Run `make check` before claiming anything is done. Reject, never downgrade
  (`PLAN.md` §2.5): never work around a failed fixture or silently downgrade a
  claim.
- Never claim something is verified that you did not actually run.

## 7. Milestones and process

- Work within the current milestone. The milestone table
  (`IMPLEMENTATION.md` §3) is the roadmap; the current milestone is the first
  entry whose acceptance tests (`core/tests/mN`) are not yet green on this
  branch — never read from a literal in this file. The root `MILESTONE` file
  is the released-milestone marker the CI image tag is derived from
  (`make milestone` validates it). A feature outside the current milestone is
  a discussion, not a change (`CONTRIBUTING.md`).
- Growth is declared, not modified (`IMPLEMENTATION.md` §4.1): a new slot or
  frontend never touches the core; a core change to add a slot is a design
  violation to review.
- Product decisions surfaced by the work go to `PLAN.md` §11 — with the
  user's approval — never only in code. Follow `CONTRIBUTING.md` and
  `CI-CD.md` for how changes land.

## 8. Environment and operations guard

Read, explore, and verify freely: inspect the tree, read the docs, run the
check recipe. But ask the user before **big changes**:

- Adding dependencies, downloading binaries, or installing tools.
- Rebuilding the container image (`ENVIRONMENT.md` §3).
- Stopping/restarting the core or any slot, or otherwise touching running
  services/containers (including `make run`).
- Git operations that mutate history or remotes (init, add, commit, push,
  merge, rebase, reset, tag, config, ...).
- Anything outside the checkout.

`opencode.json` enforces many of these mechanically (protected-file edits and
risky bash commands prompt for approval).

## 9. Secrets discipline (non-negotiable)

- No secrets in the repository. Test credentials come from `.env`
  (gitignored); `.env.example` documents variable *names* only
  (`ENVIRONMENT.md` §6, `CI-CD.md` §4).
- Never log, diff, or commit vault/decrypted material. A leaked secret is a
  failure, not a warning.

## 10. Improvement ideas

- When you have ideas for improving the project, show them to the user with
  examples and a proposed approach, and check with the user on how to proceed
  before implementing.
- If the right fix is a docs change, follow the protected-file process in §4.

## Global rules

- Don't pin specific versions of software outside of `.tool-versions` and
  language dependency files (e.g. `go.mod`, `go.sum`, lockfiles).
