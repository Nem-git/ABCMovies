# Vision

## What ABCMovies is
ABCMovies is a **self-hosted media hub**. It brings together a person’s own, legitimate access to media across many sources into one place: a single catalog to **search and browse**, and a single place to **watch, download, and manage access**.

It **aggregates and orchestrates** access — it does not create or host content, and it does not bundle accounts or secrets.

## Goals
- **One catalog per user.** Each user gets one coherent library assembled from the sources and shared access they can reach.
- **Watch and download.** Play media from a source through the user’s chosen interface, and download titles when appropriate, adapting to what the source provides and what the user/device needs, and where the source and access permit it.
- **Access under user control.** Let people connect their own accounts/access, and share the *use* of access with others in a controlled way. The operator may provide host-managed access, but ownership, limits, and revocation must be explicit. Users remain in control of accounts they linked.
- **Extensible by design.** New sources and new interfaces can be added over time without redesigning the whole system. No single “one true way” to connect a source or present the library.
- **Honest and least-surprising.** The system should not pretend (e.g., show things you can’t actually use, or silently guess in ways that corrupt your library). Limits and risks are stated plainly.

## Non-goals
- **Not a content source.** It does not provide, host, or invent media, and it does not ship with accounts, credentials, or keys.
- **Not a guarantee of availability or legality.** What’s available depends on the user’s sources and their terms; operating the system lawfully is the operator’s responsibility.
- **Not locked to one source, interface, or vendor.** Specific sources, clients, and technical choices are decided separately; technical decisions live in the decision log, not in product docs.
- **Not “everything at once.”** The rewrite defines what needs to be done and captures ideas; sequencing and scope are managed in the project tracker, not prescribed here.
