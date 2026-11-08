# ADR 0008: Config and secrets

- Status: Accepted
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #22
- Affects: local setup, deployment, source credentials, logs

## Context

The app is self-hosted and will handle source credentials/session material. The rewrite already has a strict no-secrets-in-repo rule.

## Decision

Keep configuration **file/env based for local development and self-hosting**, with committed **example files containing names only**. Real credentials/tokens/session material are supplied outside the repo (env, ignored local config, or approved secret storage). Never log or return secrets.

## Options considered

1. Env + example config — simple and portable for self-hosting.
2. Secret manager first — stronger in some environments; too heavy for Build 1.
3. Database-only config — possible later; awkward for bootstrap.

## Consequences

Foundation must define the exact config path/format and `.gitignore` coverage. Source credential custody remains a product/security decision (DEC-003) beyond basic config handling.

## Follow-up

Align with #20 and DEC-003/#16; add config examples during foundation with protected-file approval.
