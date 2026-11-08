# AGENTS.md

This file is the contract for AI agents working in this repository. It governs
how an agent develops the codebase and how it interacts with the human user.
Read it fully before doing anything.

## 1. Orientation: the docs and decisions are the source of truth

- This project is **ABCMovies**, a self-hosted media hub being rewritten.
  It is *specified before it is built*: the documents in `docs/` describe the
  product, and `decisions/` records durable decisions. The codebase implements
  what has been decided; there is no "obviously right" design that is not
  written down or explicitly approved.
- Before doing work, read `docs/README.md`, then `docs/VISION.md`,
  `docs/REQUIREMENTS.md`, `docs/PRINCIPLES.md`, and the relevant journey,
  quality, security, or operations doc. Check `decisions/` for accepted or
  proposed decisions that affect the work.
- Document precedence:
  - Product requirements and principles live in `docs/`.
  - Technical choices live in `decisions/tech/`; product decisions live in
    `decisions/product/` (or the accepted decision record they link to).
  - A GitHub issue or discussion may propose work; the durable outcome of a
    decision is its accepted decision record.
  - The GitHub project tracks work status and ordering, not product truth.
- Do not guess. If the docs and accepted decisions do not answer a question,
  ask the user, then record the answer in the appropriate place.

## 2. Ask, don't assume

- Never assume the developer's wants or how the project should be designed.
  Ask questions — as many as needed — and give examples with simple
  explanations where possible.
- Treat open questions and proposed decisions as questions for the user, never
  as decisions you make unilaterally.
- **Explain, don't cite.** Paraphrase the substance; do not quote internal
  anchors or IDs and expect the user to know them.
- **Push back.** Offer reasoned pushback on ideas — the user's and your own —
  with tradeoffs and alternatives.

## 3. Decisions and values have one home

- Product docs state *what* and *why* in plain language. Concrete technical
  choices, exact values, pins, protocols, formats, limits, and tool versions do
  not belong in product docs unless the product itself requires them.
- Every value and choice lives exactly once. Before writing a number or
  technology, find where it already lives — or should live — and reference that
  home instead.
- Homes (use the ones that exist): technical decisions → `decisions/tech/`;
  product decisions → `decisions/product/`; tool/version pins → the project's
  pin files; operational guidance → `docs/OPERATIONS.md` or the accepted ops
  decision; contract/schema values → the schema/contract that owns them.
- **Never repeat a value "for convenience."** Propose the fix via the
  protected-file process in §4.

## 4. Protected files: never edited without explicit approval

The following may not be edited without the user's explicit approval, given
after you state *what* you want to change, *why*, and provide before/after
examples:

- `docs/` — product/reference documents.
- `decisions/` — accepted/proposed decision records and their index/template.
- Contract/schema definitions (for example `proto/`, OpenAPI, or equivalent),
  once they exist: additive changes are reviewed as additive; breaking changes
  require a new version/compatibility plan and an accepted decision.
- Version pins and build/CI definitions, once they exist: `.tool-versions`,
  lockfiles' pin sources, `Containerfile`/`Dockerfile`, `Makefile` or task
  runner config, CI config (`.github/workflows/`), dependency automation config.
- Config and secrets templates: `*.example`, `.env.example`, sample config.
- `AGENTS.md` and `opencode.json`.

"Explicit approval" means the user has said yes to the concrete proposal, not a
vague "go ahead".

## 5. The code must match the docs and accepted decisions

- Verify that behavior matches the docs and accepted decisions. A mismatch is a
  problem to resolve with the user — never resolved silently by picking a side.
- Resolve a mismatch by: changing the docs/decision through the protected-file
  process in §4, or changing the code and checking with the user that the code
  reflects what was wanted.
- Record answers that are not already written down in the appropriate document
  or decision record.

## 6. Testing and verification

- Follow `docs/QUALITY.md` and the Definition of Done for the current work.
- Prefer tests that prove behavior: acceptance examples, negative cases, and
  end-to-end journeys for user-visible flows. A requirement without a way to
  verify it is not done.
- When a standard check recipe exists (CI, task runner, test command), run the
  same recipe locally before claiming work is done. Never invent a substitute
  that hides failures.
- Reject, never downgrade: do not work around a failing check or silently reduce
  what is claimed.
- Never claim something is verified that you did not actually run.

## 7. Build order and GitHub workflow

- Work is tracked in the GitHub project. Respect the current build/release
  scope: Build 1 is the first usable build; Build 2 is the first release.
  History and other deferred items may be described in docs but are not
  implemented until scheduled.
- A change outside the current scope is a discussion, not a silent
  implementation.
- Product or technical decisions surfaced by the work become decision records
  and/or decision issues — with the user's approval — never only in code.
- Follow `CONTRIBUTING.md`, issue templates, and the PR template when they
  exist.

## 8. Environment and operations guard

Read, explore, and verify freely. Ask the user before **big changes**:

- Adding dependencies, downloading binaries, or installing tools.
- Building or rebuilding images/containers, or changing CI/runtime environments.
- Starting/stopping services, containers, or long-running processes.
- Git operations that mutate history or remotes (init, add, commit, push,
  merge, rebase, reset, tag, config, ...).
- Deleting large parts of the tree, resetting generated state, or anything
  outside the checkout.

`opencode.json` may enforce some of these mechanically.

## 9. Secrets discipline (non-negotiable)

- No secrets in the repository. Test credentials come from `.env`
  (gitignored) or approved secret storage; example files document variable
  *names* only.
- Never log, print, diff, or commit credentials, tokens, private keys, session
  material, or decrypted vault data. A leaked secret is a failure, not a
  warning.

## 10. Improvement ideas

- When you have ideas for improving the project, show them with examples and a
  proposed approach, and check how to proceed before implementing.
- If the right fix is a docs or decision change, follow the protected-file
  process in §4.

## Global rules

- Don't pin specific versions outside the project's approved pin files.
