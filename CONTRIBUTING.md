# Contributing

ABCMovies is being rewritten from a clean spec. Read [`docs/README.md`](docs/README.md), [`AGENTS.md`](AGENTS.md), and relevant accepted decisions before contributing.

## How work flows

- Work is tracked in the GitHub project. Respect the current scope: **Build 1** is the first usable build; **Build 2** is the first release. Items documented as Later are not implemented until scheduled.
- Use requirement IDs (`REQ-*`) and decision IDs (`DEC-*`/ADR numbers) in issues and PRs where relevant.
- Product/technical decisions need an accepted decision record; do not leave important decisions only in code or PR comments.

## Pull requests

- Keep PRs focused. Link the issue(s) they implement.
- Run the project’s standard checks before opening a PR when they exist. CI should run the same checks.
- Include tests or acceptance evidence for behavior changes. Cover negative/error and permission cases where relevant.
- Update docs/decisions when behavior or decisions change.

## Secrets (non-negotiable)

- No secrets in the repository. Use `.env` (gitignored) or approved secret storage; example files contain names only.
- Never commit credentials, tokens, private keys, session material, or decrypted sensitive data. A leaked secret is a failure.

## Protected files

`docs/`, `decisions/`, contract/schema definitions, build/CI/pin files, config/secret templates, `AGENTS.md`, and `opencode.json` require explicit approval with a concrete before/after proposal. See `AGENTS.md`.
