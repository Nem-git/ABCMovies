# Contributing

This project is specified before it is built, and its documents are designed to stay reusable. Read [PLAN.md](docs/PLAN.md) §1 and [SCOPE.md](docs/SCOPE.md) before contributing, and follow the process rules below.

## Process

- **Milestones, not ad-hoc features.** Work is organized around milestones M0–M9 ([IMPLEMENTATION.md](docs/IMPLEMENTATION.md) §3). A feature outside the current milestone is a discussion, not a PR.
- **Fixture-first.** A requirement without a fixture is not done ([IMPLEMENTATION.md](docs/IMPLEMENTATION.md) §1.1). Code lands with its fixture suite, including negative fixtures for anything that accepts input.
- **No core changes to add a slot.** Adding an adapter is writing an adapter + fixtures + config; if it requires a core change, that is a design violation to review ([IMPLEMENTATION.md](docs/IMPLEMENTATION.md) §4.1), not a normal step.
- **Product changes go to PLAN.md.** If work surfaces a product decision, it is recorded in PLAN.md §11 — never left only in code or fixtures ([IMPLEMENTATION.md](docs/IMPLEMENTATION.md) §3). Implementation choices go to [TECHNICAL-DECISIONS.md](docs/TECHNICAL-DECISIONS.md); scope changes go to [SCOPE.md](docs/SCOPE.md) with a sign-off.
- **Reject, never downgrade.** A slot that fails its fixture suite is rejected, never silently downgraded ([PLAN.md](docs/PLAN.md) §2.5).

## Pull requests

- Short-lived branches; the PR is the unit of review ([CI-CD.md](docs/CI-CD.md) §6).
- Every PR runs the full pipeline; the conformance gate (fixtures) is a required check.
- Run the verify recipe locally before opening a PR — CI runs the identical recipes ([ENVIRONMENT.md](docs/ENVIRONMENT.md) §2).

## Secrets discipline (non-negotiable)

- No secrets in the repository. Test credentials come from `.env` (gitignored) or CI secret storage; `.env.example` documents variable *names* only ([ENVIRONMENT.md](docs/ENVIRONMENT.md) §6, [CI-CD.md](docs/CI-CD.md) §4).
- Vault/decrypted material never appears in logs, diffs, or artifacts. A leaked secret in a PR is a failure, not a warning.

## Definition of Done

Every change meets the Definition of Done in [IMPLEMENTATION.md](docs/IMPLEMENTATION.md) §6 — especially: fixtures pass, storage behavior tested per class, CI green, load-bearing contracts unchanged (or the change deliberately recorded), and the vault/secrets suite green.
