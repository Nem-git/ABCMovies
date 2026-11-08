# Glossary

Use one term per concept in docs, issues, and UI copy.

| Term | Meaning | Avoid |
|---|---|---|
| User | Any person interacting with the product. | account (unless meaning credentials) |
| Guest | A logged-out device/session identity, if supported. No durable personal state unless explicitly decided. | anonymous user (use consistently once decided) |
| Member | A logged-in user with normal access to their reachable library and personal state. | normal user |
| Admin | A member with instance-management permissions. Not automatically allowed to see other users’ private history/credentials. | superuser, root user |
| Operator | The person who installs/runs the instance. May be an admin. | host (ambiguous) |
| Account owner | The member who linked an account; a relationship, not a role. Can share/revoke that account as allowed. | owner role |
| Instance | One running ABCMovies deployment and its data/config. | server (ambiguous) |
| Host | The machine/environment running the instance. Use carefully; prefer instance/operator. | — |
| Source | An external service or library that provides catalog/access (e.g. a Jellyfin server). | provider (avoid until defined) |
| Connector | Code/integration that talks to a source. Separate from the source itself. | adapter/slot (legacy terms; avoid in product docs) |
| Account | A user’s access credentials/session for a source. | login (ambiguous) |
| Access | The ability to browse/play/download based on accounts and shares. | permission (be specific) |
| Share | An explicit grant to use access, not ownership or secrets. | credential sharing |
| Title | A movie, series, or other media entry as presented to users. | item (be specific) |
| Availability | Whether and where a title can be accessed for a user now or recently. | online status |
| Library | The catalog a user can reach, assembled from their own/shared/allowed access. | global catalog |
| Personal state | Per-user history, saved items, progress, settings; private by default. | user data (too broad) |
| Job | Long-running work with status/progress/failure/cancel where appropriate. | task/background process |
| Interface | A user-facing client (web UI first). | frontend (technical; use when needed) |
