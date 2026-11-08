# Journeys

Key flows used for acceptance examples and end-to-end tests. Each journey names the happy path and important failure/empty states.

## Build 1

- **First run / admin setup:** operator starts instance, creates first admin, reaches admin area. Failures: already initialized, weak/invalid input, storage/config error.
- **Add Jellyfin source:** admin/member links account, system verifies access, source appears usable. Failures: unreachable server, invalid credentials, expired session, permission denied — no secret exposure.
- **Sync and browse:** library syncs; user browses/searches reachable titles. Failures: source unavailable, partial results, empty library, stale availability.
- **Title detail:** shows title metadata available from the source and where it can be accessed (reachable only). No dead-end actions.
- **Play:** user starts playback of a reachable, non-DRM title. Failures: unavailable, access changed, unsupported/protected format (honest message), transient playback error.

## Build 2

- **Invite/manage member:** admin creates/invites member; member can log in and see only their reachable library.
- **Share access:** account owner grants use to a member; member gains only granted access.
- **Revoke access:** owner/admin revokes; future access stops; in-progress behavior follows DEC-004. Member’s personal state remains private.
- **Host-provided account:** admin links account and shares it; users can use it as granted; ownership/limits are explicit.

## Later (documented, not scheduled)

- Continue watching/history, downloads, enrichment, guests, subtitles selection, live/recording, third-party connectors.

## Cross-cutting failure states

Source down; title removed everywhere; session expired; access revoked; long-running work cancelled/interrupted; partial data; unsupported/protected content. Failures are clear, actionable, and never silent. No secrets in messages or logs.
