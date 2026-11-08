# ADR 0005: First integrations

- Status: Accepted
- Type: Technical
- Date: 2026-11-08
- Owners: TBD
- GitHub: #22
- Affects: Build 1 scope, connector design

## Context

Build 1 needs one real source to validate the product. The prior plan started with Jellyfin and used TMDB for enrichment.

## Decision

Keep **Jellyfin** as the first source integration for Build 1 and the **web UI** as the first interface. Treat **TMDB/external enrichment** as **not in Build 1/2** (deferred), even if the technical direction remains plausible.

## Options considered

1. Jellyfin first — user confirmed; good self-hosted/local-library fit.
2. Generic source abstraction first — cleaner long term; too much design before a working slice.
3. Include enrichment early — richer UX; expands scope and external dependencies.

## Consequences

Build 1 can focus on connect, sync, browse/search, and play. Connector boundaries should not hard-code Jellyfin assumptions into product docs, but implementation can optimize for the first working integration.

## Follow-up

Open a later decision/spike for enrichment source(s), API keys/terms, and privacy/scope if enrichment is scheduled.
