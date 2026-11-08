# ADR 0004: Storage baseline

- Status: Accepted
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #22
- Affects: config/state, accounts, library cache/state, jobs

## Context

Build 1 needs durable local state for a self-hosted instance: admin/users, connected source/account metadata, synced library data, and operational state. The prior plan used SQLite.

## Decision

Keep **SQLite** as the initial default storage baseline for Build 1 unless a concrete requirement proves it unsuitable. Keep storage access behind clear boundaries so later stores/backends can be introduced if needed.

## Options considered

1. SQLite — simple self-hosted deployment, local durability, good Build 1 fit.
2. Postgres first — stronger multi-service/concurrency story; heavier setup for a household self-hosted Build 1.
3. Pluggable database from day one — flexible; premature complexity.

## Consequences

Optimizes for easy installation and a small household deployment. Concurrency, migrations, backup/restore, and secret-at-rest handling need explicit design as features arrive.

## Follow-up

Decide migration/backup strategy before Build 2 if release hygiene is pulled in; otherwise record the operational risk.
