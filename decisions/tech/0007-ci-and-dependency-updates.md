# ADR 0007: CI and dependency updates

- Status: Accepted
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #22
- Affects: CI, repository maintenance

## Context

The prior project used GitHub Actions and Renovate. Build 1 needs CI early; dependency automation can come after there is something to maintain.

## Decision

Keep **GitHub Actions** as the CI platform. Defer dependency automation (Renovate or equivalent) until the repo has real manifests/pins worth updating. When added, it must run through normal review/CI and never bypass protected-file approval for pins/build config.

## Options considered

1. GitHub Actions now, dependency automation later — low noise, fits reset state.
2. Restore full old CI/Renovate immediately — fast, but tied to removed tooling.
3. Choose CI later — delays Build 1 quality gate.

## Consequences

Foundation work should add a minimal CI workflow that runs the project’s standard checks. `.github/workflows/` changes will require explicit approval.

## Follow-up

#20/#21 define the initial checks; add dependency automation after manifests exist.
