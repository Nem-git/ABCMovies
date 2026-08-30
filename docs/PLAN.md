# Plan

This document fixes the design decisions, the interfaces the system exposes, and how the parts work together. It is a living spec; the decision log (§11) maps each decision to where it is argued, so the reasoning isn't lost.

## 1. What the project is

### 1.1 What it is

The project is a self-hosted media hub. It gathers many streaming services into one place: a single catalog to search and browse, and a single place to watch, download, and manage access. Each service contributes what it has, and the project presents it as one library. It orchestrates the user's own legitimate access to content; it does not invent content and does not bundle secrets/accounts.

### 1.2 Goals

- **Aggregate** — bring together content from any number of sources: streaming websites, command-line downloaders, local video files, databases — anything that has video and can be talked to.
- **Enrich** — add rich metadata (posters, descriptions, ratings) from external catalogues wherever possible, never as a requirement.
- **Stream and download** — play video from any source in any interface, adapting it on the fly; download any title in the format, resolution, and container the user chooses. For DRM-protected titles, download entails decryption (see §6.6).
- **Manage access** — each user links their own accounts, can share the *use* of them with others, and can use accounts the host provides.
- **Extend** — everything is a slot that can be filled differently. New sources, capabilities, and interfaces are added without touching the core.

### 1.3 Design philosophy

- **It is a service, not an app.** The core is a capability. Interfaces — web, API, CLI — are clients stitched onto a stable core API.
- **Everything is optional and swappable.** No part of the system is the "one true way." Every feature is a slot.
- **Sources and clients can speak any language.** The service translates.
- **Minimize host power.** The host is a plumber, not a landlord: it provides the pipe and should not learn about its users from what it carries. This promise is scoped and honest: *core and operational logs are content-blind*; arbitrary slot code is trusted like any other dependency and inevitably sees what it carries. The host also necessarily sees search and enrichment queries — those must be sent to the providers in the clear. Keys and their custody are stated plainly in §7.6 — the guarantee is process discipline and explicit disclosure, not mathematics.

### 1.4 What it does not do

It does not invent content, and it does not bundle secrets in the codebase (secrets may be *vaulted* at an instance — these are different things). DRM removal is confined to an isolated slot (see §6.6). **Lawfulness is the operator's responsibility** — the system provides audit and scoping but no enforcement; this is a documented acceptance of risk, not a feature.

## 2. Foundations

### 2.1 Contracts are schemas

The system is a set of **contracts** — defined message shapes and operations — plus **transports** — the ways those messages travel. Contracts are **schemas**; the schema definition is the single source of truth. A contract says *what* a component does; a transport says *how it is reached*. Anything can implement a schema, in any language.

**Transports are an open, extensible set**: in-process, subprocess, network, or anything added later. The transport for each slot is a per-slot config value. The core is transport-agnostic: it knows the contract and reaches the slot however its config says. It never assumes.

The one constraint: **in-process carries a language constraint** (the slot must be loadable into the core's runtime). Subprocess and network are language-agnostic. Subprocess supervision (restart, backoff) is a per-slot operational detail, only for slots configured as subprocesses — not a core feature.

There are two directions of traffic, and they are different machinery:

- **Outbound (slots).** Things the core reaches out to, drives, and supervises. This is the registry and the handshake (§3, §4).
- **Inbound (frontends).** Things that reach *in* to the core's API. The core exposes an API server; frontends are its clients (§8). Frontends are not slots and never enter the registry.

### 2.2 Users and identity

An instance has **users**. Identity is local by default: a **username** and a password. Usernames are unique per instance; **the instance does not store email addresses** — so there is no email-based password recovery; the recovery path is the user's recovery key (§7.6). An optional external identity provider is a pluggable transport. Members of a shared account are simply users who have been granted access.

- The library a user sees is theirs alone (§5.1).
- Per-user settings (metadata opt-outs, quotas) hang off the user.
- Member-scoping is an invariant: member A can never observe member B's sessions, history, or quota.

**Guests.** Anonymous access is a **device identity**, not a user account. A guest can browse and search through provider anonymous/guest sessions and host-provided accounts (§5.1), and play or download through them, subject to host policy; delivery sessions for a guest reference a `guest:<deviceId>` actor for quota and audit. A guest carries no durable state: no slots owned, no watch history or playlists, no accounts owned or shared. The per-user merged library cache applies to guests too, keyed `guest:<deviceId>` with a device-session TTL (§5.1); guests are subject to guest-specific rate limits and a per-instance guest concurrency cap, so a guest can never mint device identities to exceed policy (§7.2). Account-lifecycle actions are owner-only (§7.5), so a re-auth needed by a host-provided account a guest is using surfaces to the host/owner, never to the guest. Member-scoping holds trivially: a device sees only what its own reachable sessions can serve.

### 2.3 The data model

**LibraryEntry** — a merged, canonical entry for a movie or series:

- kind: movie | series
- an immutable ID (identifiers block below)
- a **coverage map**: which providers carry the title (optionally with provider season ranges, displayed but never asserted)
- external identities (provenanced — §5.3)
- a reference to structured content metadata (best-effort — §5.2)

Seasons and episodes are **not** library entries. They are provider-native sub-items shown inside a series after the user picks a provider; no correspondence is ever asserted between providers' episode lists.

**Identifiers.** Three identifier kinds are kept deliberately separate — they are *not* interchangeable:

- **LibraryEntry ID** — `le_` + a high-entropy random value (size fixed by TECHNICAL-DECISIONS.md §1.17). Opaque, immutable, never derived from content. The merged entry's key. If a provider's item is remapped to another entry, the old entry is aliased and never destroyed; it stays resolvable while soft-hidden (§5.4).
- **Provider item ID** — a scoped key `provider:nativeId` (e.g. `providerA:123`). Stable across refreshes, but it is **availability only**: it locates an item on a provider, it is never a basis for merging (§5.3).
- **External identity** — a *set* of namespace claims, not a single ID: multiple external identity namespaces can coexist on one LibraryEntry, each with provenance (which catalogue slot supplied it) and a verdict (corroborated / single-source). Matching on **any** namespace agreement is evidence; corroboration across namespaces strengthens the verdict (§5.3).

The **coverage map** is `map<providerId, { present: bool, seasons?: number-ranges, displayOnly: true }>`: it asserts *presence*, never identity. A provider entry whose title the map says is carried is displayed **only** unless identity proof corroborates it — coverage is never taken as a merge signal by itself. When two providers look like the same title but identity proof is missing or contradictory, that is a **merge-conflict**: the entries stay apart, a `merge-conflict` event is emitted (§9.2), and the system never merges silently.

**MediaSource** — the one object that answers "what is a playable thing?". Defined in §6.2. It is a **manifest**: a structured set of per-track sources. It carries *content* and *how to fetch it*, never account identity.

**Sessions — two different objects with two different lives:**

- **Account session** — a durable, vaulted provider login (a token or session cookie held on the user's behalf). Lives in the vault (§2.4). Can be shared by use (§7.1).
- **Delivery session** — the transient representation of one play or download (§6). It carries the account context the engine needs to police, audit, and revoke: `{ provider, accountId, memberUserId, policy, providerCap }`. The engine keeps a live session→account index so revocation, quotas, and mid-stream failure handling can always map a running delivery session back to the account that owns it (§6.1, §7.1, §7.2).

**Job** — the representation of all long-running work (§9.1). A delivery session is a kind of job.

**Event envelope** — the wrapper around every notification (§9.2).

**TitleMetadata** — the structured content metadata for a title, one record per title in the metadata cache (§2.4). Contributed by catalogue slots (preferred, globally authoritative) and provider adapters (fallback, per-account) with field-level merging and per-field provenance (§5.2). The LibraryEntry references the global TitleMetadata via `metadata_ref`, not a copy. Defined in the same proto file as LibraryEntry; its schema is equally frozen once approved.

These six — LibraryEntry, MediaSource, Capability+handshake (§3), Job, Event envelope, TitleMetadata — are **load-bearing**: they must be finalized before implementation, because almost everything else bolts onto them. The two session objects are not counted separately: a delivery session is a kind of job, and an account session is a value stored under the vault contract (§2.4). Tracks are not a seventh object: they live inside the MediaSource contract (§6.2). This is the canonical definition: elsewhere in the documentation, **the load-bearing contracts** means exactly this set.

### 2.4 Storage

Stores are classified by durability (can we afford to lose it?) times who may read it. Concretely:

| Store | Durability | Who reads | Notes |
| --- | --- | --- | --- |
| Account source cache | Durable; rebuildable for library-class providers, best-effort for lazy ones | Host | Per (streaming-provider account, provider): what that account can see. Rebuildable from the provider for **library-class** providers (whole-catalogue sync); **best-effort, rebuild-by-usage** for lazy providers with no index (§5.4). *Keyed by the streaming account, not by the instance user* — every member and guest who uses that host-provided account shares the same cache |
| Identity store (provider item registry) | Durable; not rebuildable without full identity re-resolution | Host | Per (provider slot, native item id): the `{ entryId, proof }` mapping (§5.3), plus the canonical LibraryEntries and their never-destroyed aliases. Carries identity **proof only**, never coverage; losing it forces whole-library identity re-resolution |
| Metadata cache | Durable, safe to rebuild | Host | Per title (global, shared): one TitleMetadata record per title (structured content metadata with per-field provenance, §5.2); an external-ID-to-record lookup map for resolution; contributed by catalogue slots (preferred) and provider adapters (fallback); rebuildable from sources. *Global across all users and accounts* |
| Content-key cache | Durable, safe to rebuild | Host | Per (provider, contentId): DRM content keys — the key is a property of the content, not of who licensed it, so the cache is global. Entries carry only key material (encrypted at rest) and validity; TTL = license validity. A **hint, not a guarantee**: invalidated by fail-fast re-license on decrypt failure, never by credential rotation (§6.6) |
| Vault (tokens/sessions) | Durable, must not lose | Owner's key, at rest | Account sessions encrypted with per-session keys, wrapped by the owner's KEKs and a host-held relay key (§7.6); losing it logs everyone out |
| Watch history / playlists | Durable, user only | User's key | Per-user encrypted (§7.6) |
| Derived library cache | Durable, safe to rebuild | The user it serves | Per-user merged library (§5.1, §5.4); rebuilt from source caches + enrichment |
| Job/session state | Checkpointed | Host (ops) | Loss recoverable via queries |
| Event bus | Ephemeral | Subscribers | Notifications only (§9.2) |
| Audit/usage | Minimal | Host (ops) | Volume and timing, not content (§7) |

**Caches cache metadata and keys only — media bytes are never cached.** No store above holds audio/video data; a "download" is a delivery session (§6), not a copy into a cache. This is the rule that keeps an instance small and keeps provider media out of the host.

There is **no global catalog store.** The merged per-user library is *derived* — computed from the user's account source caches plus the metadata cache — and the merged result **is cached per user** (§5.1, §5.4). The account source cache avoids re-scraping providers on every library view (scraping is slow and provider-hostile); the metadata cache is global because the same title has the same metadata for everyone. The content-key cache is global for the same reason: a title's keys on a provider are the same for every user who licenses or decrypts it there.

Choose durability by regeneration cost: the source (for library-class providers), enrichment, derived-library, and content-key caches can be rebuilt — cache them; lazy-provider source caches are best-effort (rebuild-by-usage, §5.4). History can't — keep it safe. A lost notification is free to lose because the job object is queryable — keep the bus ephemeral.

### 2.5 Conformance

Every contract version ships a **fixture suite**: a fixture is a schema descriptor plus sample requests and the responses the schema says they should produce. Suites may include **negative fixtures** — requests the schema must reject — so a slot cannot pass by being vacuously permissive. To be admitted, a slot must pass the suite of the exact version it declares. **A failed claim is a rejection, never a downgrade**: nothing is guessed, downgraded, or "worked around"; the registry records the slot as `rejected` with the reason surfaced in the registry, and the operator re-declares or fixes the slot. Built-in adapters get the suite as CI. Contract drift is caught at startup, not in production. **DRM is exempted from live fixtures**: decrypting real content cannot be a fixture; the DRM slot is admitted on contract-only (mock) fixtures — covering the license-negotiation message shapes (challenge, response, keys, §6.6) — plus a manual verification path.

## 3. Slots and contracts

### 3.1 Slot types

Five slot types exist. **Every slot answers the meta-contract** (§3.3); providers just have a larger capability set.

"Slots are a fixed set" means the *kinds* above are fixed as a taxonomy; the set of *declared instances* of each kind is open (operators and, for user slots, members declare as many as they like, §4.1). A novel kind is a deliberate, documented break to this document, not an incremental addition.

| Slot | Capabilities | Contract | Notes |
| --- | --- | --- | --- |
| **provider** | search, browse, produce-sources, offer-downloads, link-account, account-session-portability | §3.2 | version per capability |
| **catalogue** | lookup-title, get-metadata | its own | enrichment source (external catalogues); declares rate limits and cache policy in its handshake response |
| **sink** | accept-bytes, resume | its own | byte destination (§3.6, §6.4) |
| **subtitle-source** | get-subtitles | its own | external subtitle source, keyed by external identity (§5.3); provenance-marked, opt-in (§6.2) |
| **drm** | acquire-keys, decrypt | its own | license negotiation + decryption (§6.6) |

**Frontends are not slots.** A slot is something the core reaches *out* to, declares in config, handshakes at startup, and supervises. A frontend is the opposite: it reaches *in* to the core's API. The core must not know a frontend exists, and adding one never changes the core (§8). Sinks are slots because the engine actively drives them; frontends drive themselves. The test: *who reaches out to whom?*

### 3.2 Provider capabilities, per operation

Capabilities are tracked **per operation**, always:

- search
- browse
- produce-sources
- offer-downloads
- link-account
- account-session-portability (see §7.4)

`produce-sources` returns a **manifest** — a structured track set (§6.2); track selection is an engine-side step downstream of the capability.

"Can this provider surface media" is shorthand for "implements search *or* browse". A provider that can browse but not search is valid. The registry records a **version per capability**, not one version per slot.

### 3.3 The handshake and meta-contract

The **handshake** is the slot telling the registry its capabilities and versions at startup — "I speak search v3, browse v1, sources v2." The slot does not send a schema; the schema is the shared contract, the slot only *declares* what it speaks.

The **meta-contract** (`CapabilityQuery`) is the contract of contracts: the first message to any slot. Every slot must answer it, in any transport. "Nothing is assumed; everything is asked" is only true because this message exists.

### 3.4 Versioning rule

One rule for all messages: **additive changes (new fields) never bump a version; consumers must ignore unknown fields. Breaking changes require a new contract version and a new handshake.** This applies uniformly to RPC contracts and event contracts. The schema encoding preserves unknown fields, which makes the additive rule native.

A slot **declares only versions it has passed the fixture suite for** (§2.5) — declaration and verified behavior must agree. Wire-additive is not behavior-compatible: a change that is additive in the wire format but alters behavior in practice (a field whose presence changes meaning for consumers that read it) is still breaking and needs a new version. A consumer that ignores the new field has not implemented the new version and must declare the version it actually implements.

**Pre-release exemption.** Until the first release ships, built-in slot contracts are exempt from the version-bump rule: their consumers are entirely in-repo and updated atomically in the same change, so breaking evolution without a new version is accepted for them. The exemption ends at first release; from then on the rule above applies without exception.

### 3.5 Account linking

Linking is described by two independent axes — not one login method:

**Axis 1 — who holds the credential (custody):**

- **Server-held (default).** The instance stores it and uses it. Simplest for users. Mitigated by §7.6: encrypted at rest, in-memory only during use. Provable? No — policy.
- **Owner-sidecar (opt-in).** The owner's process holds the credential and does the login; the host only ever sees the resulting session. Provable zero-knowledge. Costs: the owner must run the process, and members' streams egress through the owner (§7.4).

**Axis 2 — what the credential/session mechanism is (per provider):**

- **Provider username+password** — the instance logs in for you when needed. Simplest; host holds the secret; re-logins may hit captcha.
- **Cookie/session capture** — you log in once in a controlled browser; the session cookie is captured. No password stored; cookies expire, needs a human re-capture.
- **Scoped-token / API-key** — a standard scoped token where the provider offers it. Rare among streaming services; common for APIs.
- **Per-request credential** — credentials attached to every request (e.g. a standard per-request auth header). Stateless; only for services designed that way.
- **Manual paste** — user pastes a cookie/token string. Power-user fast path.

Each provider adapter **declares which mechanisms it supports**; custody is a per-account choice, defaulting to server-held.

The common mechanisms above are a **closed common set**: browser-capture, scoped-token, per-request credential, cookie-paste, and password. Anything outside that set is carried as an **opaque, provider-defined auth method** — a structured descriptor published in the adapter's handshake — and only that adapter can interpret it; `link-account` carries a `method` discriminator so a generic frontend can render an appropriate input even for a method it has never seen. Unknown methods degrade gracefully: a generic frontend renders the descriptor's own affordances rather than a bespoke form.

**Interactive login is not the core's job.** Captcha completion, email-code entry, and browser-based login are *frontend* responsibilities: the frontend runs the interactive flow and hands the resulting session material to the core through `link-account` / `update-account-session`. The core validates client-supplied sessions by probing the provider before vaulting them — it never vaults material it has not confirmed works. Some providers bind sessions to a **registered device** — a pairing flow that registers the instance as a device on the account; where the adapter declares this, **device-binding rules apply**: re-link implies re-registration of the device, and when the account's device binding is re-provisioned, the core emits `account-session-expired` for every session that was bound to the replaced device (§9.2). These flows are what a frontend runs. See §7.5.

### 3.6 The byte-stream transport (sinks)

Sink slots need two channels:

- **An RPC control channel** — start, pause, resume, cancel, finalize, report progress. Ordinary contract traffic.
- **A byte channel** — the media bytes themselves.

The engine pushes bytes; the sink flow-controls (backpressure) — **push-with-backpressure is the sink contract**. Not every sink wants push, though: **byte-addressable sinks** (browser buffers, file sinks) pull instead, requesting segments or byte ranges through **relay URLs the engine grants**, scoped to the session and validated against the manifest's locations and expiry (§6.2). For pull sinks the engine fetches only what the sink's requests demand — it never speculatively pre-fetches from the provider. One delivery session drives exactly one sink (§6.4). Per-slot transport config: the byte channel follows the slot's configured transport; transports are an open set (§2.1). Everything else about sinks lives in §6.4.

## 4. The registry

The registry is the wiring cabinet. On startup and on config reload it reads the config, reaches every declared slot through its configured transport, performs the handshake (§3.3), and keeps a live table of what is up, which capability versions it speaks, and what it can do.

- **Slots are declared, not discovered.** Every slot is declared explicitly — in operator config, or, for user slots, through the core API (§4.1) — and is **verified at add-time**: handshake, then fixture suite, then review. There is **no ambient discovery**: the instance never scans the network or filesystem for slots, so nothing uninvited ever becomes a provider.
- **Isolation.** A broken or incompatible slot is marked degraded and isolated. **The isolation guarantee holds for subprocess and network transports only** — those are crash-proof boundaries. An in-process slot shares the core's runtime; a segfault or OOM in it can take down the core, so in-process slots are best-effort and reserved for trusted first-party code. Within that boundary, a degraded slot never takes down the rest of the system.
- **Reconnect.** If a slot's process restarts or a network slot reconnects, the registry re-runs the handshake and re-admits it; until then it sits in the degraded state. A reconnect is not discovery — the slot was already declared in config.
- **Graceful drain.** On reload, new requests use the new handshake; existing sessions stay pinned to their old transport/version until they naturally end; after a grace period, stragglers get a warning event and a clean cut. Never silently kill a live session on reload. Old and new slot versions therefore run **concurrently** during drain: the reload procedure must keep old subprocesses alive, and teardown of a replaced version happens only after its sessions have drained or the grace period ends.

### 4.1 User slots

Members can declare their **own slots** at runtime through the core API, without touching operator config. The whole add path is the same as config slots — handshake, then fixture suite, then review — so a user-declared slot is verified before it is admitted, never trusted on declaration.

User slots differ from operator slots in *attribution and scope*, not in kind:

- **Attribution and rate-limiting.** A user slot is attributed to and rate-limited to its declaring user (the member whose API declared it); its load counts against that member's policy (§7.2).
- **Tenancy scope.** A user slot reaches only that member's own accounts and sessions — it can never touch another member's data or accounts, and its outputs are attributed back to the declaring member.
- **Transport.** Subprocess and network transports only. User slots never run in-process.
- **Revocability.** The declaring member can revoke their own slot; the operator can kill-switch any slot (§4).

**Least-privilege forwarding is universal — for operator slots too.** Every slot runs under the least privilege sufficient to its job (§7.6), not just user slots; the difference is that user slots add attribution, tenancy scope, transport restriction, and revocability on top of that baseline. Sharing a user slot with others is a §7.1 owner-grant with the geometry of §7.4.

## 5. Metadata and identity

### 5.1 The library is per-user

**The library is derived from the user's reachable sessions — nothing more.** It is built from the providers that user can actually reach: their own accounts, shared accounts, host-provided accounts, and anonymous/guest sessions where a provider allows browsing without a login. If a user has no account on a provider, that provider contributes nothing to that user's library.

Consequences:

- If the library only contains what you can access, there is nothing to click that you can't watch. No dead-end clicks.
- Merging (the same movie or series from many providers appears as one entry) happens within the user's accessible set, at movie/series level only (§5.3).
- Enrichment decorates entries that exist; it does not create entries.

**The merged library is cached per user** — including guests, keyed `guest:<deviceId>` with a device-session TTL (§2.2). Every browse or search runs a merge across the user's account source caches plus the metadata cache; the merged result is stored per user so the merge is not recomputed on every request. The cache is invalidated by availability events (§5.4) and by refresh-job completion; availability events are **account-scoped** (§9.2), so when a shared account's availability changes, every member and guest whose library derives from it is invalidated — no user's cache goes stale because another member's session refreshed it. A missed event leaves a slightly stale cache until the next periodic rebuild — acceptable, because the underlying source caches are equally stale on the same cadence (§2.4).

### 5.2 Enrichment

Enrichment is **best-effort and never required**; its sources are optional catalogue slots, disabled by default. External catalogues are never contacted unless the operator enables them. The operator sets which catalogues are enabled at instance level; each user can override (opt out of catalogues or specific metadata kinds).

**Multiple catalogue slots can be enabled at once, and can all run for the same item.** Each contributes different fields (posters from one, episode guides from another) with best-effort **field-level merging** — no single catalogue owns an entry — and each additional corroborating source strengthens the identity verdict (§5.3). Enrichment output is attributed per field to its source catalogue. Provider adapters can also contribute metadata fields (filling gaps the catalogue did not cover); catalogue data is preferred (globally authoritative) and provider data fills the rest.

Enrichment is **decoupled from identity**: it works without matching, using external IDs where providers carry them and heuristics over provider metadata otherwise. Enrichment improves display; it never gates identity. Enrichment data is cached per title, globally (§2.4). Entries are differentiated by their immutable LibraryEntry ID (§2.3); external IDs are informative identity (a set of claims), never the object's key.

Enrichment output is stored as a **TitleMetadata** record per title in the metadata cache (§2.4). Each typed field carries per-field provenance attribution (which slot supplied it), enabling field-level merging across catalogues and providers without a single owner. Catalogue-specific or provider-specific fields not covered by the typed schema are stored in the extra map on TitleMetadata. The LibraryEntry carries a reference to the global TitleMetadata via `metadata_ref`, not a copy.

One deliberate nuance: enrichment **may inform identity, though it never requires it**. An external ID discovered by enrichment can be adopted as a matching key (§5.3), which is why the provenance of every external ID is tracked. A home-made video is a library entry with no external match — nothing breaks.

Fallback chain: enabled catalogue → provider's own metadata → heuristics → as-is.

### 5.3 Matching

Matching decides that two provider items are the same title. **It applies only at movie and series level — never to seasons or episodes.** Episodes are provider-native and never merged or cross-corresponded.

**External identity is the primary key.** The system's first move is to resolve every provider item to a canonical external ID wherever possible. Matching then keys off that ID. Provenance matters:

- **Provider-supplied external ID** — the provider itself asserts an external ID in its metadata. This is authoritative: a matching provider-supplied ID alone is sufficient to merge. It is not a heuristic; it is an identity assertion.
- **Heuristic-resolved external ID** — enrichment resolved the item to an external ID by searching a catalogue from title+year. This is a guess, cached globally, and a wrong guess propagates to every user via the global metadata cache. So: heuristic-resolved IDs are stored with their provenance, require **at least one corroborating signal the first time they drive a merge**, and can be purged by the operator.

**When there is no external ID** (enrichment disabled, or genuinely unknown content), matching falls back to heuristics over provider metadata, conservatively:

- **Normalization.** Titles are normalized before comparison: Unicode normalization, lowercase, diacritics stripped, common leading articles ("The", "A", "An"; the list is configurable) dropped, whitespace collapsed. Year is compared exactly for movies.
- **The merge rule:** merge iff normalized title + year are equal **and** at least one corroborating signal matches exactly. Corroborating signals, ranked: director → shared cast member → original language → duration. (Duration is the weakest signal — theatrical and director's cuts differ.)
- **Otherwise, they stay separate.** Conservative matching favors keeping unlinked duplicates over wrongly merging two different films. Duplicates are accepted as a cost.

**Immutable entry IDs.** Merging is a union of availability onto one canonical entry, never an ID destruction. Watch history, playlists, and downloads reference stable entry IDs, so a later merge is lossless. When two entries merge, the losing ID must still resolve — it is aliased to the canonical entry, never destroyed. No user correction feature exists for now; identity is purely algorithmic + provider IDs. Errors are accepted and permanent (a merge/split tool is a possible future addition).

**Provider item registry.** The `(provider, providerId) → { entryId, proof }` mapping is **durable state, not a per-request computation**. `proof` is the evidence that fixed identity at resolution time: the provider-supplied external IDs plus title and year. An **availability refresh is a pure lookup** in this registry — it re-checks presence on the provider and updates the per-account availability that feeds the coverage map (§5.4), and performs **no identity work at all**. Identity resolution (matching §5.3, including any external metadata queries) runs only on three triggers: the item is **first-seen**; the provider's identity-relevant metadata changed (title, year, or provider-supplied external ID differ from proof); or the system needs **corroboration** for a heuristic verdict. If resolution yields a proof that disagrees with the stored one, the item is **never silently remapped**: it becomes a new instance, a `merge-conflict` event is emitted (§9.2), and the registry keeps both until a corroborated resolution settles it. This registry is also what makes provider IDs "availability only" (§2.3) concrete: the provider ID locates the item, the registry says *which entry* that location currently belongs to.

**Recycled provider IDs.** Streaming providers recycle item IDs. The rule: a provider ID is a key into the registry, never identity — when the registry detects the item under a known ID has changed so much that its proof no longer matches (new external IDs, different title), it is treated as a new item and the old mapping is retained as an alias until proven dead.

**Entry-level correction is designed-for, not implemented.** Splitting a wrongly-merged entry, or burying a duplicate, is a deliberate future feature (§11) — the data model already supports it (aliased, never-destroyed IDs; provenance retained on every external ID; coverage maps per entry). Nothing in the plan depends on it being present on day one, and implementing it must never change identity semantics for entries that were never corrected.

**Provenance retention is mandatory.** Every external ID, every coverage claim, and every merged-mapping decision retains its provenance — which slot supplied it and when — for audit and for later correction. Provenance is not optional metadata; it is the material a future split/bury tool would need.

**Coverage.** Merged entries carry a coverage map recording which providers carry the title; provider season ranges may be shown as unasserted provider information. Because season/episode correspondence is never asserted, browsing a series means picking a provider and using that provider's native episode list.

The coverage map is one row per provider. A row:

```text
"providerA:host" {   present: true, seasons: [[1,3],[5,5]], verdict: "corroborated",
                via: "account:providerA:host", lastVerified: <ts> }
```

| Field | Meaning |
| --- | --- |
| **key** | Which provider slot this row is about (`provider:nativeId`-style scoped key) |
| `present` | Does the provider carry the title right now? From the last sync/usage (§5.4). The headline answer: can I press play here? |
| `seasons` | Series only. Which seasons the provider has, as **inclusive ranges, non-contiguous** (S1–S3 and S5). Episodes are never listed. Unasserted — display info, never merged |
| `verdict` | Identity confidence on *this* row: `corroborated` (identity proof exists — a matching provider-supplied external ID, §5.3) vs `presence-only` (we only see a same-named item; shown but never trusted) |
| `via` / `lastVerified` | Provenance. Which reachable account observed it, and when. The same provider differs per account (a host-provided library vs a guest's anonymous view), which is what makes the map **per-user** (§5.1) — each user's merged library holds *their* projection |

**`displayOnly` holds for the whole map.** The coverage map is a label for humans; it **can never, ever drive identity or merging** — it displays, it does not decide. That is why presence (this map) and identity (the registry above) are kept separate: the map can be cheap, lazy, and possibly stale without ever corrupting the merged identity.

### 5.4 Churn

Content leaves services constantly. Availability is what feeds the coverage map (§5.3) and soft-hide (below), and it is obtained by **exactly three routes — never by probing titles one-by-one**, because per-title probing is how an account gets flagged for scraping:

- **Whole-catalogue sync** — for **library-class providers** (self-hosted library servers) that answer "what's in this account's library?" with a list. The scheduler fetches the item index once per cadence and diffs it against the registry: encountered items are new, missing ones left the provider. Cost is driven by **pagination, not entry count** (2000 items ≈ ~20 paginated calls). This is normal client behavior, not scraping, and it is the *only* case that warrants a scheduled job.
- **Lazy / usage-confirmed** — for streaming services with no clean index. **No background probing.** Browse/search against the provider *is* the refresh (results feed the diff against the registry); `produce-sources` at click/play confirms availability and updates the cache. Dead-end protection is enforced at resolve time, not by a crawl.
- **On-demand** — a *user-driven* resolution for a specific entry. Never a background job over the whole set.

**Scheduler rules.** Per-provider cadence is **declared in the adapter's handshake** (as the catalogue slot declares rate limits, §3.1); the scheduler runs within it. Add jitter + a minimum-interval floor so N accounts sharing one provider don't synchronize; **exponential backoff on provider errors or 429s is a per-provider signal, not per-account** — one throttled account throttles all traffic to that provider; a **per-provider aggregate pacing governor** (config-declared, §7.2) bounds total load across all accounts on that provider; **pause entirely** while a provider is marked degraded (§4). Background refresh, enrichment, and user traffic all draw from the **same per-account pacing budget** (§7.2) — background work can never exceed, or bypass, the limits user requests obey. Availability refresh is a **pure lookup** (§5.3): it changes presence and the per-account availability that feeds coverage, never identity.

**Uses.** Availability drives four things: the "Available on…" list (coverage map), soft-hiding a title that left every provider, provider-preference selection (§6.5), and dead-end prevention — the last enforced **live at resolve time**, so stale coverage is a display hint, never the gate that admits a dead end.

**Honesty note.** For lazy providers, coverage is **best-effort and may be stale** — the plan does not promise otherwise.

A provider dropping a title emits an **account-scoped** availability-changed event (§9.2), which invalidates the merged caches of every member and guest whose library derives from that account (§5.1) and re-runs the merge on next access. The entry persists with an updated coverage map. Removal is a metadata event, not a data deletion.

**Empty coverage.** When a title leaves every provider, its entry is **soft-hidden**: excluded from browse and search, but kept resolvable by ID so history, playlists, and downloads do not dangle. History rows for departed titles render as "unavailable" — no dead-end clicks in browse; departed titles may still appear in history.

## 6. The delivery engine

### 6.1 Overview

Playback and download are the same operation with a different destination: a **delivery session** that pipes bytes from a provider to a sink. Both run the same flow — **resolve a manifest → deliver its tracks** (§6.2) — with the goal governing how much of it is delivered: play delivers the *whole menu* and lets the player choose; download delivers the user's chosen *recipe* into a container (§6.3, §6.4).

Each session has a **goal** — *play* (bytes to a player, discarded) or *download* (bytes to a persistent sink). The goal is a hint that seeds defaults.

**Pipeline precedence: user request > instance config > delivery goal > engine default.** A higher level may only override the one below it. If the user's explicit choice is incompatible with the source, degrade to the nearest supported pipeline and surface a warning — never fail silently, never override the user's explicit pick.

**Account context lives on the delivery session, not on the MediaSource.** Every delivery session carries `{ provider, accountId, memberUserId, policy, providerCap }` (§2.3), so the engine can always enforce quotas, attribute audit, and find sessions to kill on revocation.

### 6.2 The MediaSource contract: the manifest

The MediaSource is the **manifest**: a structured set of **tracks** that answers "what is playable" for one provider's version of a title. It is **self-sufficient** — anyone downstream can act on it without asking the provider again — and it is the **unit of internal consistency**: everything in one manifest comes from one master (one cut, one timeline), so its tracks always play in sync. **Tracks are scoped to their manifest; selection never spans manifests**, and cross-provider failover (§6.5) re-resolves a fresh manifest rather than reusing tracks.

Manifest-level fields:

| Field | Meaning |
| --- | --- |
| `type: static\|live` | Finite content, or an endless live stream |
| `seekable: none\|full\|dvr-window(span)` | Declared per-source capability, reported by the adapter — not a truth about the title. Providers differ on the same title; live may be seekable only within a DVR window |
| `addressable: PER_TRACK\|WHOLE_MUX` | **Container-level**: the fetch unit. `PER_TRACK` = each track has its own locations (§6.2 delivery fields); `WHOLE_MUX` = the whole muxed container is the fetch unit — tracks carry full descriptors plus `carried_in` references instead of per-track locations. Demux (§6.3 remux/transcode) is applied only when the sink cannot consume the mux as-is |
| `tracks[]` | The track set below |

Tracks are grouped by media type, each carrying its media properties:

| Media type | Properties |
| --- | --- |
| **video** | codec, bitrate, resolution, frame rate, HDR range |
| **audio** | codec, bitrate, language, channel layout, role (main / descriptive / commentary) |
| **subtitle** | format, language, role, forced |

Subtitle `role` is drawn from a standard subtitle-role vocabulary — `subtitle`, `sdh`, `signs-songs`, `easyreader`, `commentary` (default `subtitle`) — plus a `forced` boolean (applies only to the `subtitle` role). **Burned-in subtitles are degenerate subtitle tracks**: they carry language, role, and forced only, and no delivery fields — they are metadata describing what is already baked into the video. The roles map to the standard role/characteristic attributes of the relevant streaming and container formats. Note: `main` is a valid role for *audio*, not for subtitles — a subtitle track never declares `main`.

Each track also carries delivery fields — the shape the atomic MediaSource used to have:

| Field | Meaning |
| --- | --- |
| `locations[]` | One or more URLs/endpoints that can serve that track's bytes (fallbacks and quality variants). **Optional**: present for `PER_TRACK` and for side-car tracks; absent for tracks carried inside a mux |
| `carried_in` | For `WHOLE_MUX` manifests: a reference to the container track that carries this track (e.g. the video mux holding audio+subtitle). When present, the track has **no** per-track `locations` — the container is fetched and demuxed if the sink needs it split |
| `authContext` | How to use those URLs — cookies, headers, tokens to attach. **Engine-side only: it is never forwarded to sinks or frontends, and never logged.** |
| `drm?` | DRM system + license URL, when encrypted. Optional; absent for most tracks |
| `expiry` | When the URL stops being valid (stream URLs expire in minutes-to-hours) |
| `refreshPath` | A reference to the provider operation that yields a fresh source when `expiry` hits. **Engine-side only**: the engine calls it; frontends get a session-level refresh verb, never the raw provider method |

Per-track delivery is **non-exclusive**: a track may have `locations`, `carried_in`, or both. **Mixed manifests are first-class** — a manifest with muxed video+audio and side-car subtitle tracks is not an edge case. Delivery follows selection (§6.2): a play session serves every track in the delivered menu; a download session fetches every track in its recipe.

**Selection is a distinct step — and differs by goal.** For **play**, selection is deliberately *minimal*: the engine picks **one version** (one manifest, §6.5 provider preference), then hands the sink **the whole menu** — every audio track, every subtitle track, and every video rendition the target's device-profile can decode (codec/container compatibility, so everything offered is playable). Mid-stream audio/subtitle switching and bitrate (ABR) decisions are then the **player's** job, not the engine's; the engine does not decide "which language" or "which quality." For **download**, selection is a **recipe** the user composes (§6.3): one video track, N audio tracks (main + commentary), N subtitle tracks, chosen by target. A delivery session holds one sub-source per selected track; the session and job objects (§2.3) are unchanged.

Example (abridged):

```text
{
  type: "static", seekable: "full", addressable: "PER_TRACK",
  tracks: [
    video:     [codec], 1920x1080, 5.8 Mbps, 23.976 fps, SDR, locations [.../1080], expiry "30m", refreshPath ...
    video:     [codec], 1280x720,  2.8 Mbps, 23.976 fps, SDR, locations [.../720],  ...
    audio:     [codec], en, 5.1, role main,               locations [.../en51],          ...
    audio:     [codec], en, 2.0, role descriptive,        locations [.../desc],          ...
    subtitle:  [format], en, forced false,                locations [.../en],            ...
    subtitle:  [format], es, forced false,                locations [.../es],            ...
  ]
}
```

A user asking for "1080p, English, subtitles on" as a *play* session gets the whole menu: both video renditions (for ABR), the en 5.1 audio, and the en subtitle — the player's interface then lets them switch to the 2.0 descriptive audio or the es subtitle **mid-stream** because every track was delivered. A *download* recipe ("1080p · en 5.1 · en subs") selects those three tracks only and composes them into a file.

**Mux and demux.** Delivery fields already record the container shape (§6.2, `addressable`); how that shapes playback:

- **Play.** The sink/engine pair decide how to present the source based on what the **sink declares it can accept** (§6.4). If the sink consumes separate tracks (a standard adaptive-streaming player), the engine **demuxes** a `WHOLE_MUX` container into its component video/audio elementary streams and re-serves them as distinct adaptation sets — so the player sees the same thing it would from a `PER_TRACK` manifest. Demux is a **transparent compatibility step only**: split the container (or re-mux to a fragmentation-friendly container), **never a re-encode**. If the sink consumes the container natively (a file player, a media server), **no demux** — hand it the container stream; `carried_in` + media properties still populate its menu. Text subtitles inside a mux demux to their sub stream; **burned-in subtitles stay degenerate tracks** (language/role/forced, no delivery) so the UI knows they're baked into the video.
- **DRM before anything.** If the mux is encrypted, `decrypt` (§6.6) runs first; demux/remux operate on clear (or decrypt-to-clear) streams.

**External subtitles.** A subtitle-source slot (§3.1) returns subtitle tracks keyed by the title's external identity (§5.3). They are **provenance-marked** (`provider-native` vs `external`), **opt-in** ("also search external subtitles"), and surfaced with an honesty note: a different cut's timing may not match. `provider-native` subtitles are the standard where a provider carries them; external subtitle sources are the fallback, keyed by the same external identity, and `sdh`/`signs-songs` roles are honored when the source declares them.

### 6.3 Pipelines and targets

The user does not pick an encoded track — there is no encoded-track catalog; the space of possible encodes is unbounded, so enumeration is meaningless. The engine presents **targets**: a resolution, a codec, a container, a max bitrate, or a device profile. Each target resolves by rule:

- **Native first.** If a manifest track qualifies for the target, deliver it **passthrough**. Always the first choice: free, lossless, zero server load, least detection risk.
- **Encode only as fallback.** If no native track qualifies (target below the provider's floor, or a codec/container/bitrate the provider does not serve), encode from the best qualifying native track. **Encoding never upscales** — native is the ceiling; you cannot create detail that is not there.
- **Feasibility bounds access.** Whether a target is satisfiable is computed (native ceiling) and bounded by operator policy (encode-resolution caps, disable-encode, per-account limits). Every target reports `satisfied: native | encoded` so interfaces can surface server cost.

Pipelines:

| Pipeline | Does | Used by |
| --- | --- | --- |
| **passthrough** | Deliver a native track as-is | play, download |
| **remux (per-track)** | Re-container a *single* stream without re-encoding; a compatibility step; also **selects/drops streams** (e.g. remove unwanted audio) while re-containerizing | play (when the player cannot consume the native container) and download |
| **transcode** | Encode from the best qualifying native track to the target; never upscale | fallback when no native track qualifies |
| **compose** | Assemble the selected, finalized tracks plus target container into one deliverable | download only — playback delivers streams, it does not materialize a file |
| **record** | Capture a live source, gap-handled, stop → finalize | live |

**A pipeline is an ordered step-chain, not a single kind.** A session runs the steps in order (each step's output feeds the next; a step may name explicit inputs to fan in from earlier steps). The engine builds the chain from the goal and manifest, records it on the session at start — it is the decision record and the seed of the resume key — and validates every step is executable before the session runs. A step the engine cannot genuinely perform (DRM decrypt, transcode, live record) is **refused loudly and logged**, never silently downgraded to a passthrough. v1 selects only passthrough, container-copy remux (with stream selection), and compose — the chain for a whole-mux download is remux → compose; a play is a single passthrough, or a single remux when the sink needs a different container.

Defaults: play → passthrough when compatible, otherwise remux for compatibility or transcode; download → passthrough/remux where possible, compose last (unless the user chose loose per-track files). Subtitle conversion and SDH stripping are optional per-track transforms. Pipelines are named and swappable.

**Downloads are a multi-track recipe, not a single stream.** A download is one file, and the chosen container holds many tracks (multi-audio and multi-subtitle support depend on the target container: the common formats carry multiple audio tracks and select subtitle codecs). The user's recipe lists the tracks to include — e.g. video 1080p + main audio + commentary audio + original subs + commentary subs — and remux/compose select exactly those into the output file, dropping the rest. Nothing forces one-of-each. For `WHOLE_MUX` downloads the container largely *is* the deliverable: rename and (if the user wants) remux to the target container to add/remove tracks — no re-encode. Multiple audio and subtitle tracks survive trivially because they are carried through, not collapsed.

**Targets are rule-based, not scored.** A future extension is **scoring profiles** (attribute→weight profiles: give me "~20GB, a modern codec, 7.1, SDR" without an exact spec); it is documented as an idea, not implemented here. The target contract keeps additive room (§3.4) so a profile mechanism can be added without a breaking change.

**Pipeline selection is separate from scheduling** (§6.5): what pipeline is chosen is independent of when it runs.

### 6.4 Sinks

Sinks are a slot (§3.1, §3.6). A delivery session pipes bytes to a *sink*, and sinks are pluggable. **One delivery session drives exactly one sink** — the session's byte channel (§3.6) has a single consumer, so backpressure, revocation, and audit stay unambiguous. The user's device is the built-in sink; anything else — instance storage, a cloud target, a peer-to-peer sharer — is a custom sink added without touching the core. Each sink implements the control and byte channels (§3.6): accept bytes with backpressure, report progress, pause/resume/cancel, finalize. Each sink declares what it can accept; sink discovery is a delivery-side concern, not a provider capability. Sinks are declared in instance config, with per-user defaults; *choosing* a sink for a given download is a frontend concern.

The same manifest may back several concurrent sessions, each to its own sink; each session counts independently against policy and caps (§7.2). Multi-sink fan-out is **not** upstream coalescing: the engine does not deduplicate provider fetches across sessions (possible future optimization, not promised).

**Player sinks are an illustrative example, not a requirement.** One example of a sink is a **shared media-server sink**: an adaptive-streaming origin that relays the engine's bytes to many players with per-session tokens. Such a sink emits the **full menu** for the session's version — all audio adaptation sets, all subtitle sets, all playable video renditions — translated from our track descriptors into the standard attributes of the target streaming format (bandwidth, dimensions, codecs, resolution, frame-rate, video-range, language, channels, forced/caption flags, and the standard audio/subtitle roles), so the player's own language, subtitle, and quality selectors work with no provider-specific knowledge. **The sink's declared capabilities decide demux-vs-passthrough** (§6.2): a sink that can only consume separate tracks causes the engine to demux a `WHOLE_MUX` source; a sink that consumes the container does not. Sinks remain pluggable — a player sink is one sink implementation among many, never special-cased in the core — and its transport (in-process vs remote) is unspecified per deployment. This is the *only* sink described by the plan; nothing here requires it to exist.

**Output contract.** A download's deliverable is produced through a naming contract: a template over title and track metadata (title, season/episode, resolution, codec, language, channels, range, media type) with defined collision behavior. Compose (§6.3) may optionally attach metadata to the deliverable — fields from the metadata cache's TitleMetadata (§5.2) plus per-track technical metadata — off by default, inheriting enrichment's opt-in rules. A shipped default template exists (TECHNICAL-DECISIONS.md §1.15); it is operator-configurable, and changing the *default* is a PLAN.md §11 change.

**Recording is a sink.** The record pipeline's output (a captured live stream) is delivered the same way as any download — stream-append to the chosen sink while the live feed runs, then finalize on stop. Instance-local disk is simply one sink implementation; a cloud target is another. No separate storage concept exists.

### 6.5 Relay, and scheduling

**Proxy-only.** All bytes are relayed through the service — no direct-URL handoff. The direct path is dropped for now; MediaSource and the engine stay shaped so it could return later. Cost: all bytes cross the instance (bandwidth, CPU). Proxy egress slots (§7.3) provide egress routing disabled by default.

**Admission control.** The job queue has per-resource caps (max concurrent transcodes, aggregate bandwidth) and backpressure into the request layer. New work waits in the queue or gets a busy answer — **busy answers carry a queue position and notify when the session starts**, so "busy" is never a dead end. The system degrades gracefully instead of collapsing. **Pacing adds a rate axis** to the same machinery: per-account budgets for requests per second, concurrent pulls, inter-request delay, and retries — enforced alongside quotas (§7.2). Off by default; a host or account enables it.

**Latency budget.** Play sessions get a priority path: start immediately, first segment ASAP. Downloads go through the queue. Same engine, different urgency.

**Record pipeline (live).** A live channel isn't a file — segments expire. Record means near-real-time pulls, gap handling when the feed hiccups, and stop → finalize. Its own lifecycle.

**Seek/resume.** The declared `seekable` on the manifest (§6.2) drives UI and pipeline behavior — a DVR window bounds seek; transcoded sessions seek keyframe-aligned. Download jobs checkpoint periodically, so a restart means *continue*, not *restart*. Checkpoints use **source-native resume markers** — a media-sequence index, a segment/period+representation reference, a byte offset — so a resume never depends on the engine's own byte accounting. The resume key is `{ provider, manifestRef, trackId→marker, selectedTarget, container }` and is only valid **within the same provider and the same resolved track set**: if the manifest re-resolves differently, the job fails honestly rather than pretending the old markers apply.

**Provider preference.** When a title is reachable through several providers, the engine picks the source per a **config-driven ordering** — provider order, then quality, then language — arranged by the user or by the instance, defaulting to a documented priority. The user's explicit pick, if any, wins (§6.1 precedence).

**Mid-stream failure.** Failure behavior is pinned per case:

- **Downloads retry in place** (checkpointed) against the same provider. There is **no cross-provider failover for downloads**: provider A's file is not provider B's file, and resuming "at the same byte" is meaningless across different encodes. On persistent failure, the job fails with a clear event; partial output survives for same-provider resume.
- **Static play fails over** to another provider carrying the title: the engine re-resolves, gets a fresh manifest (§6.2), and seeks to the nearest keyframe ≤ the last position. **The engine always emits a `provider-switched` event** (§9.2) carrying the reason and the resume point; frontends filter or hide it at their discretion — the switch is never silent. Roll-forward means the playback position may drift ahead of the exact last position; that is accepted and reported, not masked.
- **Live play/record rejoins at now.** A live stream has no absolute position; a new provider is at *their* now, which is the same wall-clock time but not the same content point. Reconnect at "now" and emit a gap event where the outage happened. Record keeps the partial segments and stitches around the gap.

**Session coalescing.** Explicitly deferred. We use one upstream session per consumer; shared-account concurrency is bounded directly by the provider cap (§7.2). Coalescing — one upstream session serving N consumers — may return later; the session and MediaSource contracts keep it possible without retrofitting.

### 6.6 The DRM slot

DRM is a **required component**, implemented as an isolated, versioned slot with two operations, each versioned per-operation (§3.4):

- **acquire-keys** — given a track's drm context (DRM system, license endpoint, initialization data, and any engine-side-only auth the license call needs), return content **keys**: key IDs, material, and validity.
- **decrypt** — given a track source and its keys, return a clear, transcodeable stream.

**Decrypt-to-clear is the composition of the two** — it is the promised behavior, and what makes §1.2's "download any title in the format, resolution, and container the user chooses" coherent: the player needs no DRM module at all.

A **license wrapper** — a thin proxy that relays license requests while content stays encrypted, so the player's own decryption module still decrypts — is a *different composition* of the same operations (relay license traffic; keys never leave the player side), expressible as configuration over the same slot. Not part of current contract.

Keys are **TTL'd message values** with a validity, storable in the content-key cache (§2.4) so re-downloads reuse keys instead of re-licensing. The cache is **keyed by content** — `(provider, contentId, keyId)` — because a title's key is the same for every user who decrypts it on that provider; entries carry only key material (encrypted at rest) and validity (TTL = license validity). The cache is a **hint, not a guarantee**: on a decrypt failure the entry is dropped and `acquire-keys` is re-run (a normal call); if that fails too, the standard exhaustion path applies. **Credential-set rotation never purges the cache** — blocking a burned device stops *acquisition*, not decryption with keys already obtained, and cached keys remain valid because providers control access by licensing, not by re-encrypting media.

**Device credentials.** To acquire keys, the slot authenticates to license endpoints as a device. It holds a **set** of device credentials (vaulted, §2.4), **rotates automatically** on rejection, and raises the `awaiting-action` job (§9.1) only when the set is exhausted — the human step is interactive re-provisioning, run by a frontend. Provisioning and refresh are otherwise internal to the slot.

For DRM content, download entails decryption — i.e., DRM removal — the lawful-sensitive operation (§1.4). The DRM slot is admitted on contract-only (mock) fixtures (§2.5), which cover the license-negotiation message shapes (challenge, response, keys) but never decrypt real content.

## 7. Accounts and sharing

### 7.1 Proxy model

Accounts are **shared by use, never by credential**. An account is a live session (vaulted) plus a list of members. When a member plays or downloads through an account, the session is used on their behalf — the member never sees a secret. Sharing is owner-grants-only. All usage is auditable; the owner can revoke a member at any time. **Revocation is an engine-side kill**: the engine finds every delivery session whose `memberUserId` matches the revoked member on that account (§2.3) and ends them mid-stream; **a revoked member's in-progress downloads are discarded.**

### 7.2 Policy

Usage policy is enforced server-side and is carried as a **generic policy map** — `map<limit-type → value>` — rather than a fixed set of policy fields, so new limit kinds need no schema change (§3.4). Limit types include: concurrent streams, bandwidth caps, time windows, per-member quotas, and pacing budgets (requests/sec, concurrent pulls, inter-request delay, retries). Every instance has an **instance default policy**; the owner can **override any subset per-account**. **Policy is grounded in the provider's real limit:** the engine enforces `min(policy_limit, provider_cap)` — the adapter declares the account's upstream limit as a config value, so "policy allows 4, provider allows 2" never fails at the source. The declared cap is config, never discovered by probing: empirically testing the provider's limit risks bans. Because many accounts can share one provider, an **instance-declared per-provider aggregate governor** (max requests per second across all accounts on that provider) bounds total load, and 429/error backoff is a per-provider signal (§5.4) — per-account budgets alone cannot stop N accounts from collectively tripping a provider's limits. **One fan-out, one enforcement point**: a member's requests hit the policy map exactly once regardless of how many accounts, slots, or sessions are involved, so limits can never be bypassed by splitting work across accounts.

### 7.3 Proxy egress slots

Linked proxies are an **egress slot**: the relay can route through them, so the public IP the provider sees is the proxy's, not the instance's. Configurable, **off by default**. Uses: avoid IP bans, geo flexibility, decouple the host's IP from account activity. Composes with both custody models.

Streaming platforms fingerprint devices and IPs; **consistent egress** (one stable egress IP per account) mitigates detection. Residual risk is accepted.

Delivery sessions carry a **routing attribute** distinguishing **control traffic** (manifest, license, API — the account-identified calls where detection matters) from **bulk traffic** (media bytes from content CDNs). When egress routing is enabled, control traffic follows the account-consistent path while bulk bytes may flow direct. Disabled by default, host-enabled.

**Bulk-direct is per-provider gated, not a global toggle.** Each adapter declares whether its content CDN tolerates a split IP (control through egress, bulk direct) — many CDNs do not. When egress routing is enabled, only providers whose adapter declares CDN tolerance get the bulk-direct path; the rest keep control and bulk on the same egress route. And this is **independent of custody (§7.4)**: egress routing and sidecar custody compose, but neither depends on the other.

### 7.4 Sidecar custody

Two integration shapes exist, so the labels are defined here rather than assumed:

- **Relay-through-owner.** Every request is relayed member → instance → sidecar → provider. The sidecar needs no inbound ports; the owner's connection carries the traffic. Works with any provider, because negotiator and streamer share the owner's IP.
- **Portable-session.** The sidecar negotiates the session, then the instance streams directly to the provider. Cleaner and faster, but works only where the provider tolerates **portable sessions** — many bind sessions to the IP/device that negotiated them, so the negotiator and streamer must share an IP, or the provider must not care.

**currently ship relay-through-owner only.** **Portable-session remains a goal**: portable sessions are too rarely supported to design around, and we have not found a workable path to it yet. Feasibility of relay-through-owner (double-hop latency, owner bandwidth) will be evaluated as we build.

**Relay-through-owner has an availability coupling that is accepted and documented:** members can use a shared account only while the owner's sidecar process and connection are online. The frontend surfaces "owner offline" so members aren't left confused. (Where a provider later proves portable, that account can be moved to portable-session and the coupling drops to session-refresh time.) This coupling is specific to the **sidecar** path. In **server-held** mode, sharing does not depend on the owner being present: the vault's relay key (§7.6) lets the instance unwrap an account session for a member relay regardless of whether the owner is logged in.

**Account-session portability is a declared provider capability** (§3.2): where portable, portable-session is used; where not, relay-through-owner.

### 7.5 Session refresh

Sessions die: services expire them, log you out, or force re-login on new-device detection. For most streaming sites there is no refresh token — "refresh" means re-login, sometimes captcha or email code. Sessions have a **TTL**; when one dies, the engine requeues the owner ("your provider session expired — re-link"), a human step. No blind automated re-login.

**Re-linking is client-driven.** The core's role is: emit an `account-session-expired` event, create a `re-auth` job with status `awaiting-action` (the job payload names the account owner as the only one who may act), and expose `link-account` / `update-account-session` for the *frontend* to submit new credential material (captured cookies, pasted tokens, scoped-token results). The interactive step — captcha, email codes, running a controlled browser — is implemented by the frontend; the core validates client-supplied sessions by probing before vaulting (§3.5).

### 7.6 Host-power minimization

- **Content-blind relay.** Core and operational logs record volume and timing ("1.2 GB to user X at 21:03"), never item identity. The host gets only the data it must handle to be the pipe; content-blindness is a property of *core and ops logging*, not of arbitrary third-party slot code (§1.3).
- **Per-user encryption.** Watch history and playlists are encrypted at rest under the user's DEK, wrapped by keys derived from the user's own secrets (below). The guarantee is honest: the host must process this data (merge, jobs, serving), so it holds the unwrapping key whenever it does so. The protection is process discipline (decrypt in memory only during use, never logged), not mathematics. The host-held recovery-KEK opt-in below is the only path that works without the user's secrets present.
- **Vault.** Account sessions are encrypted at rest, decrypted in memory only during a relay, never logged, never written to disk in the clear. Labeled honestly: in server-held mode the operator *technically* can decrypt the vault — that is a policy, not a proof, and the host structurally holds a relay key that can unwrap account sessions (below). The provable zero-knowledge option is the sidecar. Server-held stays the default for usability.

**How the keys work.** Identity and data custody are separate concerns.

- **Login identity: username + password.** No email addresses are stored. (Two-factor and external identity-provider login are optional and deferred.) The password gates login; it is hashed for verification server-side. Key derivation is **server-side**: the server derives the password-KEK from the password using Argon2id and stores the hash for verification.
- **Data custody: a recovery key.** At account creation the user is shown a one-time **recovery key** — a high-entropy random string (size and encoding in TECHNICAL-DECISIONS.md §1.11) they must save (a password manager prompt). The server derives the recovery-KEK from the recovery key using Argon2id.
- Each user has a random **data encryption key (DEK)**. The DEK is wrapped by *two* key-encryption keys: one derived from the password (for daily login) and one derived from the recovery key (for recovery). Blobs are encrypted with the DEK.
- **Password change** = unwrap with the old password-KEK, re-wrap with the new. **Password reset** = the recovery key re-wraps a fresh password-KEK — data survives. Because no email is stored, **the recovery key is the only password-recovery path**, and losing it means losing the data. An instance may opt into a host-held recovery KEK for its users, which restores operator recovery at the cost of honesty about the claim ("policy, not proof").
- **Shared accounts.** Each account session is protected by its own random **session key**: the session (token or cookie) is encrypted with it, and the session key is wrapped by *three* parties — the owner's password-KEK (daily ops), the owner's recovery-KEK (recovery), and a **host-held relay key** (a vault-only key in the instance's operational keystore). A member's relay unwraps the session key with the relay key — no owner presence required — so sharing does not depend on the owner being logged in (§7.4). Owner-only actions (link, re-auth, revoke) still require the owner's KEK. The relay key can decrypt *only* account sessions, never user blobs, keeping the trust classes clean; rotating or revoking it kills every member relay at once. Each member's own history and playlists use that member's own DEK; a member never holds the owner's key.

**Trust, stated out loud.** There is no mathematical boundary; there is discipline and disclosure. **User blobs** (history, playlists, derived library) are encrypted at rest under the user's DEK, and the host holds the unwrapping key whenever it processes them — a process promise (in-memory only, never logged), not a proof. The **vault is a different class**: in server-held mode the host structurally holds the relay key and can decrypt account sessions — its protection is "policy, not proof" by design, so the instance's privacy story rests on the operator's discipline, not on encryption claims. Anyone evaluating the instance's privacy story must read both classes as trust, not encryption. The recovery story is the hinge: lose the recovery key and user blobs are gone; the host-held recovery KEK opt-in restores operator recovery at the cost of that honesty.

## 8. Frontends and communication

### 8.1 Frontends

The core exposes a stable set of application services over an **inbound API server**. Frontends — web, API, CLI — are **clients** of that API: they reach in; the core never reaches out to them (§3.1).

The core's contracts are schemas; frontends translate to their own external surfaces — a web frontend may serve its browser a different encoding even though the core speaks the contract wire format. The core thinks in sessions, jobs, and facts; interfaces adapt that to their nature.

The exact transport for the core API is an implementation-time decision recorded in TECHNICAL-DECISIONS §1.2, not part of this spec. It is chosen per the plan's transport principle: the contracts are fixed; the transport is a swappable detail. The constraint that drives the choice is the *hardest client* — a browser cannot speak the chosen transport's raw wire protocol — and the goal is **one inbound protocol**, not several dialects of the core API.

### 8.2 Two communication styles

- **Synchronous**, for questions needing an immediate answer: search, browse, start a session, link an account.
- **Events**, for facts about state: a session ended, a download finished, a provider went down.

Sync calls fan out per-provider **in parallel, with per-provider timeouts and partial-result assembly** — a slow provider degrades alone, never blocks the fast ones. A sync call **responds once**, with the results that arrived within the caller's horizon and a `{ results, pending, timedOut }` split; providers still pending or timed out are named. **Late stragglers are not lost**: when they land, the underlying job is updated (§9.1) and a completion event is emitted (§9.2), which subscribed frontends merge into their view. One rule governs both styles — each provider degrades alone, and no caller waits forever.

## 9. Events and jobs

### 9.1 Jobs are the system of record

Long work is represented as **jobs** with visible status: `{ id, status: queued|running|awaiting-action|done|failed|cancelled, progress }`. **Job-status queries are the system of record.** Every state transition is emitted as an event.

`awaiting-action` covers jobs blocked on a human step — re-linking an expired session, completing a captcha, entering an email code. The interactive work is done by a frontend, never the core (§7.5); the job carries who must act.

Actions come in two kinds, mirroring §3.5's auth methods. A **closed common set**: re-link, captcha, email-code, device-pairing — the core defines these and any generic frontend can render them. Outside that set, actions are **open passthrough**: opaque structured prompts the adapter defines and interprets, carried by the job; a generic frontend renders a fallback form from the prompt's descriptor rather than failing on an action it has never seen.

**Idempotency.** Session start takes an idempotency key; retries are safe; double-start is impossible (a double-start on a one-stream account is a real, likely bug).

**Liveness is a core-API contract.** A delivery session stays alive only while its owner proves it:

- **Play sessions** require the frontend to call `deliverySession.heartbeat()` on a fixed interval, with a server-side grace (the frontend adapter's contract; the concrete interval and grace are the config defaults in TECHNICAL-DECISIONS.md §1.14). A legitimately paused movie still heartbeats, so it is not killed.
- **Downloads need no heartbeat** — fetch progress *is* the heartbeat.

**No phantom sessions.** A session ends when: the sink finalizes, the connection drops, the heartbeat times out, the TTL expires, or it's cancelled/revoked. The concurrent-stream limit counts *active* sessions; idle silent sessions are evicted rather than blocking others.

### 9.2 Events are notifications

Events are **at-most-once notifications**, not a durable record. If a subscriber misses a "download finished" event, it recovers by querying the job — no loss. The bus is ephemeral in-memory pub/sub. Ordering and replay are not guaranteed and not required. Durable storage is reserved for the vault, history, and caches (§2.4).

**Events are tenancy-routed.** A job's event audience is its `memberUserId`; account life-cycle events (linked, expired, revoked, device re-provisioned) are owner-only (§7.5). **Availability events are account-scoped**: their audience is every member and guest whose library derives from the account (§5.1), so a shared account's refresh invalidates all affected derived caches, not just the owner's. Delivery is contingent on the subscriber's authenticated identity plus per-connection scope filters — a subscriber only sees events for entities it is authenticated to see, and a connection declares which scopes it wants. This is an authorization boundary at the bus, not a convenience flag.

## 10. Guardrails and risks

Risks are documented where their mitigations live: contract drift (§2.5, §3.4), DRM and lawfulness (§1.4, §6.6), DRM credential exhaustion (§6.6), provider fragility and isolation (§4), provider-side detection and pacing (§6.5, §7.2, §7.3), third-party catalogue outages and external-subtitle timing (§3.1, §6.2), bad enrichment identity (§5.3), vault/instance compromise and key loss (§7.6).

## 11. Decision log

Key decisions, pointing to where they are argued in the body:

| Decision | Section |
| --- | --- |
| Contracts are schemas; transports open and per-slot | §2.1, §3.4 |
| Slots are a fixed set of *kinds* (incl. subtitle-source) with a universal meta-contract; instances within a kind are open; a novel kind is a deliberate break. Frontends are clients, not slots | §3.1, §3.3 |
| Per-user library, merged and cached per user; two caches, no global catalog | §2.4, §5.1 |
| Merge at movie/series level only; external ID first; immutable entry IDs; soft-hidden empty coverage | §5.3, §5.4 |
| Proxy-only (egress routing available, off by default); failover pinned per case; session coalescing deferred | §6.5 |
| min(policy, provider cap); pacing budgets enforced server-side, off by default; a per-provider aggregate governor bounds load across accounts on one provider | §7.2, §5.4 |
| Queries are the record, events are notifications | §9.1, §9.2 |
| DRM is a dependency; decrypt-to-clear is the composition of acquire-keys + decrypt; credential set rotates, awaiting-action only at exhaustion; composition is gated by synthetic encrypted fixtures, not just message-shape mocks | §6.6, §2.5 |
| MediaSource is a manifest of per-track sources; tracks are manifest-scoped; selection never spans manifests; seekability is a declared capability (none/full/dvr-window) | §6.2 |
| Targets, not encoded-track catalogs; native-first, encode only as fallback, never upscale; compose is download-only, per-track remux serves play and download | §6.3 |
| Subtitles are a first-class track kind; external subtitles come from a subtitle-source slot; output naming contract plus optional metadata attachment | §6.2, §6.3, §6.4 |
| Content-key cache is global and keyed by content; a hint, not a guarantee — fail-fast re-license on decrypt failure, credential rotation never purges | §2.4, §6.6 |
| Identity and custody are separate | §7.6 |
| Content-blind scoped to core/ops logging | §1.3, §7.6 |
| Server-held default custody; sidecar relays through owner for now | §3.5, §7.4 |
| Vault sessions use per-session keys wrapped by the owner's KEKs and a host-held relay key; member relays don't need the owner present; the relay key never touches user blobs | §7.6 |
| Interactive login is client-side; provider device registration is one of the interactive flows | §3.5, §7.5 |
| Lawfulness is operator responsibility | §1.4 |
| Guests are device identities, not user accounts; member-scoping holds; guest libraries cache under `guest:<deviceId>` with a device-session TTL; guests get rate limits + a concurrency cap | §2.2, §5.1 |
| Three identifier kinds are separate: entry ID, provider item ID, external-identity set; coverage asserts presence, never identity; merge-conflicts never merge silently | §2.3, §5.3 |
| Provider item registry is durable state holding identity proof only — coverage lives in per-account availability and is projected per-user; availability refresh is a pure lookup; identity resolution only on first-seen / metadata change / corroboration need | §5.3 |
| Coverage provenance is multi-account (`CoverageRow.via` repeated, breaking pre-release); the provider namespace is the slot instance id, never the adapter name | §5.3 |
| Media bytes are never cached — caches hold metadata and keys only | §2.4 |
| Conformance: reject, don't downgrade; fixtures may be negative; slots declare only versions they pass | §2.5, §3.4 |
| Auth methods: closed common set + opaque passthrough; device-binding re-provisioning expires affected sessions | §3.5 |
| Slots are declared, not discovered; verified at add-time; user slots add attribution, tenancy scope, transport restriction, revocability on top of universal least-privilege | §4, §4.1 |
| Byte-addressable sinks pull via engine-granted relay URLs; the engine never speculatively pre-fetches | §3.6, §6.4 |
| Player/media-server sink is an illustrative example, not a requirement; sinks stay pluggable | §6.4 |
| Checkpoints use source-native resume markers; static-play failover always emits provider-switched with resume point | §6.5 |
| Policy is a generic map; one fan-out, one enforcement point; owner overrides subset per account | §7.2 |
| Bulk-direct egress is per-provider gated on adapter-declared CDN tolerance; independent of custody | §7.3 |
| Trust, stated honestly: no mathematical boundary — user blobs and the vault are both policy-not-proof, with exactly which keys the host holds disclosed | §7.6 |
| Sync calls respond once with partial results; late stragglers update the job and emit completion events | §8.2 |
| Job actions: closed common set + open passthrough; events are tenancy-routed and scope-filtered; availability events are account-scoped, invalidating every affected user's derived cache | §9.1, §9.2 |
| Availability is never found by per-title probing: whole-catalogue sync (library providers) + lazy/usage-confirmed (streaming services) + on-demand; one shared pacing budget; background work cannot override user limits; per-provider aggregate governor bounds cross-account load | §5.4, §7.2 |
| Coverage is a per-user display index with per-provider rows (present / seasons / verdict / provenance / lastVerified), projected from per-account availability, never stored in the identity registry; displayOnly means it can never feed identity | §5.3 |
| Play = the whole menu from the chosen version (all audio, all subs, all playable video); the player does mid-stream switching and ABR; download = the user's multi-track recipe into one container | §6.1, §6.2, §6.3 |
| Mux handling: demux is a transparent compatibility step, decided by the sink's declared capabilities, never a re-encode; DRM decrypt runs before demux/remux | §6.2, §6.4 |
| Output naming contract: a default template over title and track metadata, operator-configurable | §6.4 |
| Provider preference default: provider order, then quality, then language; the user's explicit pick wins | §6.5 |
| Play sessions stay alive by heartbeat; downloads don't (fetch progress is the heartbeat) | §9.1 |
| The built-in sink is the user's device; instance-local disk is a second v1 sink; sink deliverables are not caches | §6.4 |
| Structured content metadata (TitleMetadata) replaces flat metadata_links; one record per title with external-ID-to-record lookup; per-field provenance from catalogues (preferred) and providers (fallback); no denormalized snapshot on LibraryEntry | §2.3, §2.4, §5.2 |
| Until first release, built-in slot contracts may evolve breaking-ly without a version bump: consumers are entirely in-repo and updated atomically; the breaking-change gate stays off until then | §3.4 |
| Catalogue items carry all their content metadata inside an embedded TitleMetadata; which of those fields count as matching evidence is decided by the matching engine, never by the schema | §3.4, §5.3 |
| Pipelines are ordered **step-chains** (a DAG of transform stages), not a single kind; the engine records a session's chain at start, and a step v1 cannot run is **declined loudly and logged** (decrypt / transcode / record), never downgraded to passthrough | §6.3, §2.5 |
| Slot sink config is a namespaced `options` map (disk `path` lives under `options`); the flat `Path`/`Retention` sink fields are removed | §6.4, config |
| A play session announces itself twice: a delivery-menu-ready event (menu painted) before the job-status event that records the session; a subscriber attached as the slot owner/relayer receives both, in that order | §6.5, §8.2, §9.1 |
| A linked account becomes usable at the next provisioning of its slot, never mid-flight: at runtime, WireLink custody is vault-first only — no catalogue feed until the slot's next build | §3.5 |

**Scope of this log.** This log records *product* decisions only. Implementation decisions (language, transport, tooling) are recorded in TECHNICAL-DECISIONS.md; scope and acceptance live in SCOPE.md; feasibility evidence lives in RESEARCH.md. This document deliberately stays agnostic about all three.
