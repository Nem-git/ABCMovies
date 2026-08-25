# Technical Decisions

This document records the **implementation-specific choices** the other documents deliberately stay agnostic about. PLAN.md fixes *what* the system is; this document fixes *which technology realizes it*. It exists so PLAN.md, IMPLEMENTATION.md, ENVIRONMENT.md, TESTING.md, and CI-CD.md stay clean and reusable as references.

**Boundary:** product decisions belong in PLAN.md §11's decision log; **implementation decisions belong here**. Scope and acceptance belong in SCOPE.md. Spike/research findings belong in RESEARCH.md. A decision that changes the product is a PLAN.md change first; this document only records the technology that realizes it.

Each decision records the choice, the rationale, and the constraint it satisfies.

## 1. Decisions

### 1.1 Core language and in-process slots — **Go**

- **Decision:** the core is written in Go; in-process slots (§2.1, §4 of PLAN.md) are Go.
- **Rationale:** single static binary (simple self-hosted operations), excellent first-class protobuf support, easy subprocess supervision, strong standard library for HTTP/gRPC serving. Team familiarity is a factor — the in-process language constraint (§2.1) means the core runtime and in-process slot language must match.
- **Consequence:** subprocess and network transports remain language-agnostic (§2.1); only in-process slots inherit the Go constraint. A slot that cannot or will not be Go uses subprocess or network transport.

### 1.2 Inbound API transport — **gRPC**

- **Decision:** the core's inbound API server (§8 of PLAN.md) speaks gRPC.
- **Rationale:** the contracts are Protobuf (§1.10); gRPC is the natural, typed transport for a Protobuf service surface and gives streaming for events (§8.2) and progress reporting out of the box.
- **Consequence — the browser (hardest client, §8.1):** a browser cannot speak raw gRPC. The web frontend reaches the API over **gRPC-Web** — the same Protobuf contract in a browser-compatible wire encoding (no HTTP/JSON gateway; gRPC-Web is not a REST/JSON API and never a second dialect of the core API). The gRPC-Web termination is part of the **frontend's serving layer** — a thin wrapper around the core's gRPC server, owned and built by the frontend project (`frontends/web/`) — so the core itself stays a pure gRPC service and never changes for a browser. In v1 that serving layer imports the core in-process, so the deployment remains one process, one port.
  - This is a **reusable client-integration pattern, not an in-repo detail**: any frontend project — first- or third-party — terminates gRPC-Web in its own serving layer and reaches the core over plain gRPC; a browser never talks to the core directly, so CORS never enters the core path. Non-browser clients (CLI tools, other services) reach the core over plain gRPC with no translation layer. Clients generate their own stubs from the published, versioned contracts (PLAN.md §2.1, §3.4) and authenticate with the opaque bearer token (§1.12). The in-repo web frontend is the reference implementation of the pattern.
- **Consequence:** streaming events use gRPC server-streaming; the event bus (PLAN.md §9.2) stays in-process pub/sub, with gRPC streams as the subscriber transport.
- **Concrete realization (M0):** the serving layer composes the core through its exported bootstrap seam (`core/app`) and wraps the service implementation with **Connect** handlers (`connectrpc.com/connect`), which serve gRPC-Web, plain gRPC, and the Connect protocol from one HTTP port; the browser page uses the gRPC-Web transport explicitly. The wrapper library `improbable-eng/grpc-web` was evaluated and rejected: maintenance mode since 2023, no release since 2021. Two adapter details live in the frontend: grpc status errors are translated to their connect equivalents (the two code spaces share one numbering), and bearer-token authentication runs as a connect interceptor mirroring the gRPC interceptors' public-method allowlist. The alternative of transcoding the gRPC server directly via `connectrpc.com/vanguard` (its `vanguardgrpc` subpackage would eliminate the hand-written adapter) was evaluated and deferred: the library is alpha, and the adapter is small and fully covered by integration tests; revisit if the adapter's maintenance cost grows. Browser stubs are generated from the same schemas by `protoc-gen-es`, pinned as npm dev dependencies (versions in the lockfile, like the formatters).

### 1.3 Repository layout

- **Decision:**

```text
/
├── proto/                # .proto schemas — single source of truth (§2.1 of PLAN.md)
│   ├── abcmovies/core/v1/     # load-bearing contracts (PLAN.md §2.3)
│   ├── abcmovies/slots/v1/    # slot kinds (PLAN.md §3.1)
│   └── abcmovies/api/v1/      # inbound API service surface (§8)
├── core/                 # the core service (packages of the repo-root Go module)
│   ├── app/              # exported bootstrap seam for in-process embedders (§1.2)
│   └── tests/            # milestone tests, white-box, mirror M0–M9
├── adapters/             # built-in slot implementations (Go, one dir per adapter)
├── frontends/            # frontend clients (web, CLI)
├── fixtures/             # fixture suites: fixtures/<contract>/v<version>/ (+ handshake/, negative/)
├── tests/                # black-box suites over public seams/fixtures (reserved; empty until the first such suite)
├── docs/                 # PLAN.md, IMPLEMENTATION.md, ENVIRONMENT.md, TESTING.md, CI-CD.md,
│                         # TECHNICAL-DECISIONS.md, SCOPE.md, RESEARCH.md, THREAT-MODEL.md, OPERATIONS.md
├── Containerfile         # dev + CI image (ENVIRONMENT.md §3)
├── Makefile              # make deps / proto / web-build / fmt / secret-scan / lint / build / test-unit / vuln / check / run / run-web
├── go.mod, go.sum        # repo-root Go module: core + frontends together (§1.4)
├── .tool-versions        # canonical version pins (ENVIRONMENT.md §1)
├── package.json          # formatting/linting tooling pins: prettier, markdownlint-cli2 (§1.4)
├── .golangci.yml         # golangci-lint v2 config — gofumpt formatting enforcement
├── .prettierrc           # prettier style (JSON, YAML) and targets (.prettierignore)
├── .markdownlint-cli2.yaml        # markdownlint style and targets (Markdown)
├── config.example.yaml   # committed example of the instance config
├── .env.example          # committed names of local test credential variables
└── LICENSE               # AGPL-3.0
```

- **Rationale:** mirrors the plan's own separations — contracts (proto/), core, slots (adapters/), clients (frontends/), conformance (fixtures/), milestone tests beside their component (core/tests/). Proto directories mirror the package paths (`abcmovies.<kind>.v1`), so the schema linter's package-directory-match rule holds with no exceptions (§1.18). Milestone tests are white-box — they verify stored bytes and internal state — and Go's internal-package rule confines imports of `core/internal/*` to `core/`, so they live under `core/tests/`; the top-level `tests/` tree hosts black-box suites over public seams when one exists. Empty placeholder directories are not tracked; a directory in this tree materializes with its first real content (e.g. `adapters/jellyfin/` at M1).

### 1.4 Version pins

- **Decision:** pins live at **one home per tool kind**, never duplicated: the core language pins itself via its manifest's `toolchain` directive (auto-enforced on every tool invocation), and the schema tooling + linter live in `.tool-versions` at the repo root, referenced by the Containerfile and CI (ENVIRONMENT.md §1, CI-CD.md §3). A change to a pin invalidates caches (CI-CD.md §8). Go is not listed in `.tool-versions` because the Go toolchain cannot be version-switched on hosts where developers already manage Go; the manifest's `toolchain` directive (GOTOOLCHAIN=auto) makes the manifest authoritative anyway, so listing Go twice would reintroduce the exact split-brain this rule prevents.
- **Initial pin set** (re-verified at M0 scaffolding, IMPLEMENTATION.md §8.4): Go **`go 1.26` / `toolchain go1.26.6`** in the repo-root `go.mod`; **buf 1.72.0**, **golangci-lint v2.12.2**, **node 24.19.0** (runtime for the formatting tooling), and **gitleaks 8.24.3** (secret-leak scan) in `.tool-versions`; as Go tool dependencies in the repo-root `go.mod` (executables installed to `bin/` by `make deps`, §1.6): protoc-gen-go **v1.36.12** (always equal to the `google.golang.org/protobuf` runtime version), protoc-gen-go-grpc **v1.6.2**, protoc-gen-connect-go (§1.2), grpc-go **v1.83.0**, and **govulncheck v1.7.0** (vulnerability scan, `make vuln`). The node-based tools are npm dev dependencies — the formatters (`prettier`, `markdownlint-cli2`) at the repo root, and the web codegen/bundling tools (`protoc-gen-es`, `esbuild`, §1.2) likewise at the root — pinned in their `package-lock.json`; their versions live there, not in `.tool-versions`.

### 1.5 Instance config location and secrets convention

- **Decision:** the instance reads configuration from a single YAML file at `config/instance.yaml` (gitignored), with a committed example at `config.example.yaml` and variable *names* documented in `.env.example` (ENVIRONMENT.md §6, CI-CD.md §4). Values never enter the repository.
- The vault's encryption keys and the host relay key (§7.6 of PLAN.md) are generated locally, never committed, never logged.

### 1.6 Make recipes

- **Decision:** the Makefile is the single entry point, matching ENVIRONMENT.md §5 and CI-CD.md §1 exactly — CI never invents its own commands.

| Recipe | Runs |
| --- | --- |
| `make deps` | language-level dependencies from the pinned versions (Go tools, `npm ci`) |
| `make proto` | regenerate code from the `.proto` schemas (Go + web stubs) |
| `make web-build` | bundle the web frontend (`frontends/web`) into its embeddable `dist/` |
| `make fmt` | rewrite formatting in place — `buf format`, gofumpt, prettier |
| `make check` | lint + build + full suite (unit, round-trip, integration, fixture suites) + vuln — the CI gate |
| `make run` | boot the skeleton (registry, slot, API server) |
| `make run-web` | boot the web frontend's serving layer (core embedded in-process, §1.2) |
| `make lint` / `make build` / `make test-unit` / `make vuln` | the individual stages `make check` composes |

`make lint` enforces all formatting and hygiene mechanically: buf lint + format freshness on the schemas, gofumpt on Go, prettier on JSON/YAML, markdownlint on Markdown, and the secret-leak scan (`make secret-scan`). `make test-unit` runs the unit suites with the race detector. `make vuln` runs govulncheck against the module. CI runs these recipes verbatim (CI-CD.md §1); it never invents its own commands.

### 1.7 CI platform — **GitHub Actions**

- **Decision:** GitHub Actions. The worked example that CI-CD.md §7 used to carry is recorded here so CI-CD.md stays vendor-neutral; the pipeline stages and gates are vendor-neutral and the platform is interchangeable.
- **Stage mapping:**
  - **lint, typecheck/build** — a job per stage on a fresh runner from the pinned toolchain (the same Containerfile as ENVIRONMENT.md §3). The lint job also runs the secret-leak scan (`make secret-scan`) — the "CI secret-leak gate" THREAT-MODEL.md T12 and CI-CD.md §4 promise.
  - **schema checks** — a dedicated job running the schema lint; breaking-change detection is disabled until the first release (§1.24).
  - **unit + round-trip** — a job running the fast hermetic suites.
  - **fixtures** — the load-bearing job: runs the fixture suites for every built-in adapter; a required check on every PR.
  - **integration** — the cross-layer job, including the vault/secrets suite (never skippable).
  - **image** — built only when the prior stages pass; tagged by milestone on green main.

### 1.8 First provider adapter (M1)

- **Decision:** Jellyfin is the first library-class provider adapter (initial; confirmed by the RESEARCH spike before M1 starts).
- **Rationale:** fully open-source with a documented, stable REST API and a clean "what's in this account's library?" index — the cleanest match for PLAN.md §5.4's whole-catalogue sync.

### 1.9 Dependency updates

- **Decision:** dependency updates are **continuous** — PRs land as versions release — and **automated via Renovate**. Renovate is configured against the single canonical pin source (`.tool-versions`, §1.4) and the language manifest, with grouped minor/patch updates; it opens normal PRs that run the full pipeline (CI-CD.md §1).
- **Contracts are exempt from automation.** A contract change is a recorded decision (PLAN.md §11) with a new fixture suite (TESTING.md §3), not a dependency bump (IMPLEMENTATION.md §8.1).
- **The bot never merges.** Every update PR goes through the full pipeline and the same review discipline as any change; *reject, never downgrade* applies to dependency updates too (IMPLEMENTATION.md §8.2).
- The Renovate config (`renovate.json`) lands at M0 scaffolding alongside the frozen initial pins (§1.4), which is when the workflow becomes live (IMPLEMENTATION.md §8.4).

### 1.10 Contract schema format — **Protobuf, compiled by buf**

- **Decision:** the contracts (§2.1 of PLAN.md) are encoded as Protobuf schemas; the `.proto` definitions are the single source of truth. Codegen and schema checks run through the **buf** toolchain: `buf generate` (invoking the pinned per-language plugins below), `buf lint`, `buf breaking` (against the last approved state — satisfies the schema gate in CI-CD.md §3 when enabled; disabled until first release, §1.24), and `buf format`.
- **Rationale:** unknown-field preservation makes the additive-versioning rule (§3.4 of PLAN.md) native; first-class support in the core language (§1.1); buf carries its own compiler, so no system `protoc` binary is pinned or required, and lint + breaking-change detection ship with the same pinned tool. Per-language plugins (`protoc-gen-go`, `protoc-gen-go-grpc`) are invoked by buf as local executables pinned via the core manifest's tool mechanism (§1.4).
- **Consequence:** the reference documents (PLAN.md, IMPLEMENTATION.md, TESTING.md, CI-CD.md, ENVIRONMENT.md) speak of "schemas" generically; the concrete encoding is recorded here.

### 1.11 Crypto, normalization, and transport-security primitives

- **Decision:**
  - Server-side key derivation with **Argon2id** (§7.6 of PLAN.md).
  - User blobs encrypted with the DEK under **AES-GCM** (§7.6 of PLAN.md).
  - Recovery key: a **~128-bit base32-encoded** random string (§7.6 of PLAN.md).
  - Title normalization: **Unicode NFKD** (§5.3 of PLAN.md).
  - Transport encryption: **TLS** on all network transports (THREAT-MODEL.md T11, OPERATIONS.md §1).
  - **Pinned parameters** — frozen at M0; any change is a security-sensitive change (IMPLEMENTATION.md §8.3) and breaks already-stored blobs: Argon2id **m=19 MiB, t=2, p=1** (RFC 9106 first profile), run **server-side**; AES-256-GCM with **256-bit keys (32 B)**, **128-bit tag**, **12-byte random nonce generated fresh per encryption**; recovery key **128-bit random, base32, 26 characters**, shown once at signup.
- **Rationale:** standard, well-audited primitives; the exact algorithms are pinned with the toolchain (§1.4).
- **Consequence:** IMPLEMENTATION.md §8.3's security-sensitive-update scrutiny and THREAT-MODEL.md refer to these primitives generically; the specifics live here.

### 1.12 API authentication — **opaque bearer token**

- **Decision:** the inbound API (§8 of PLAN.md) authenticates with a single mechanism: an **opaque bearer token** presented as a wire-level `Authorization: Bearer <token>` header (or the gRPC-Web equivalent). This authenticates API sessions; the engine-granted relay capability for sink bytes (§3.6 of PLAN.md) and the vault's keys (§7.6 of PLAN.md) are separate mechanisms and never conflated with it.
- **Rationale:** one mechanism serves every client type — browser (through the frontend's serving layer, §1.2), CLI tools, and other services — and matches the "one inbound protocol" goal. Opaque tokens avoid the JWKS/issuer machinery a signed JWT would require of a self-hosted instance with no identity provider.
- **Consequence:** the sessions store holds the token's hash, never the token value; a token is minted at login and revoked per session. Authentication resolves a **principal** — today always an authenticated user (`user:<id>`); guest access (deferred to v2, SCOPE.md) would be an additional principal kind (`guest:<deviceId>`) added without changing the mechanism, and until then every non-public method requires a valid token. Delivery-session liveness is separate: heartbeat and TTL (§9.1 of PLAN.md, defaults in §1.14).

### 1.13 Delivery sinks — **two co-equal v1 sinks**

- **Decision:** v1 ships **two co-equal sinks**: the **user's device** (built-in; a browser download through the frontend) and the **instance-local disk** (declared as a sink; operator-configurable retention). Neither is "optional"; a download resolves to whichever sink the user chose.
- **Rationale:** the user's device is the only v1 sink that needs no configuration; the instance-local disk is the natural deliverable location for a self-hosted library with no client attached. Both are needed for v1 to be usable (§6.4 of PLAN.md).
- **Consequence:** the device sink needs no config declaration — its configuration is the frontend's. Disk-sink deliverables are user-requested downloads owned by the sink (OPERATIONS.md §5), not §2.4 caches. The mechanism for composing a *third* sink (remote disk, NAS, another service) is deferred; that is v2 design work, not a v1 config option.

### 1.14 Shipped config defaults

- **Decision:** the instance config ships with these defaults: sync cadence **6 h** plus jitter and a minimum-interval floor; delivery-session TTL **24 h** (the zombie cap, §9.1 of PLAN.md); play-session heartbeat interval **30 s** with a server-side grace of **90 s** (§9.1 of PLAN.md); policy defaults `concurrentStreams: 3`, `bandwidth: unlimited`, `encode: disabled`; drain grace **5 min**; streaming-service pacing **off**.
- **Rationale:** the defaults are the values PLAN.md §5.4, §6.4, §6.5, §7.2, and §9.1 name as the v1 baseline. Recording them here means `config.example.yaml` and PLAN.md can stay agnostic about concrete numbers.
- **Consequence:** every default is operator-overridable through the config file; there is no per-guest or per-device TTL (guests are deferred to v2, SCOPE.md).

### 1.15 Output naming contract — **arr/TRaSH-style template**

- **Decision:** the naming contract (§6.4 of PLAN.md) ships with an arr/TRaSH-style default template, operator-configurable:
  - Movie: `Title (Year) [Resolution] [Codec] [Audio Channels] [Language].ext`
  - Series: `Series Title (Year) - S01E01 - Episode Title [Resolution] [Codec] [Audio Channels] [Language].ext`
  - Collisions append `(2)`, `(3)`, ...; characters illegal in path names are stripped.
- **Rationale:** the arr/TRaSH convention is the de-facto standard for self-hosted media libraries (Sonarr/Radarr), so deliverables land in tools' expected format with zero further renaming.
- **Consequence:** the template is data, not code — configurable without a code change; the default is frozen for v1 and any change to the *default* is a PLAN.md §11 change.

### 1.16 Store backends — **SQLite, one file per store class**

- **Decision:** every store (§2.4 of PLAN.md) uses **SQLite**, with **one database file per store class**. The in-memory session→account index is engine-internal, not a store.
- **Rationale:** SQLite is a single-file, zero-ops engine that fits the self-hosted footprint and the "no installed services" operations goal; one file per store class keeps cache-store data clearable without touching vault or sessions data.
- **Consequence:** the vault (§7.6 of PLAN.md) is a store like any other: per-value AEAD encryption (SQLite file stores encrypted blobs), so a leaked database file yields nothing without the DEK. Non-SQLite backends remain possible per class at implementation time; the plan stays store-class agnostic.

### 1.17 LibraryEntry ID size

- **Decision:** the LibraryEntry ID (§2.3 of PLAN.md) is a **128-bit random value** with the `le_` prefix, pinned here so the load-bearing contract schema can be authored against it at M0.
- **Rationale:** 128-bit random values give collision-free, unforgeable identifiers without coordination. PLAN.md records the identifier *scheme* (opaque, immutable, never derived from content); this entry pins the *size*.
- **Consequence:** the size is frozen once the schemas ship (§2.3 of PLAN.md); changing it later would break already-stored data, so treat it like the other pinned values here.

### 1.18 Schema lint and breaking config — **buf**

- **Decision:** `buf lint` runs the **STANDARD** rule set, with no disabled rules; `buf breaking` runs the **FILE** rule set but is disabled until first release (§1.24). The proto directory tree mirrors package paths (`abcmovies.<kind>.v1`, §1.3), so the lint rule that demands a file's directory match its package name passes by construction.
- **Rationale:** a schema gate without exceptions (CI-CD.md §3) needs lint config that a contributor cannot silently widen; mirroring directories to packages removes the one rule that would otherwise need a carve-out.
- **Consequence:** the proto files' package name and their directory path are locked together; moving either one is a schema change (IMPLEMENTATION.md §8.1).

### 1.19 Fixture suite format — **JSON descriptor + JSON cases**

- **Decision:** a fixture suite (TESTING.md §3) is a JSON directory under `fixtures/<contract>/v<version>/` with:
  - `suite.json` — a descriptor: `{ "contract", "version", "kind" (one of handshake | positive | negative), "slot" (the slot under test), "transport" (defaults to the slot's transport) }`.
  - `cases/*.json` — one file per case, each a sample request with its expected response for `handshake`/`positive` kinds, or an invalid declaration the registry must reject for `negative` kind.
- **Rationale:** JSON is a language-neutral data format (the conformance gate is language-neutral, TESTING.md §2.1); one case per file keeps negative fixtures (mandatory for anything that accepts input) readable and diffable.
- **Consequence:** the format is data, not code — any adapter in any language can run a suite; the generic runner (`core/cmd/fixture-runner`, TESTING.md §3) consumes it.

### 1.20 Session DEK cache — **memory by default, sealed-store opt-in**

- **Decision:** unwrapped per-session data-encryption keys (the DEK the server holds after login to serve per-user encrypted blobs, §7.6 of PLAN.md) are cached **per session, not per user**, and live in process **memory by default** (`auth.dek-cache: memory`). An operator may opt into `auth.dek-cache: encrypted-store`, which persists entries in the sessions store sealed with the vault cipher. Either way, no plaintext key material reaches disk; an entry's lifetime is exactly its session's — revoked or expired tokens evict their key material with them.
- **Rationale:** user-keyed caches leak across sessions: a second session inherits key material minted for another login, and logout cannot evict without breaking concurrent sessions. Per-session keying makes revocation and expiry exact. Memory-only keeps the strongest default (nothing persists); encrypted-store exists for instances that must survive restarts without re-login.
- **Consequence:** with `memory` (default) a restart requires users to log in again. With `encrypted-store` and an ephemeral vault key, sealed entries are unreadable after a restart — memory-equivalent semantics; pair it with a pinned hex vault key to persist. The knob ships in `config.example.yaml`; the default is frozen for v1.

### 1.21 Provider capability mapping — **whole-catalogue sync declares `browse` v1**

- **Decision:** a library-class provider's whole-catalogue sync surface ("what's in this account's library?", §5.4 of PLAN.md) is declared at handshake as capability **`browse` version 1**. Search and produce-sources get their own capability names and are declared only when their fixture suites exist. Within the sync contract (`proto/abcmovies/slots/v1/provider.proto`):
  - every request names its target `account_id` from day one — adding it later would be wire-additive but behavior-breaking, since old adapters would answer any account with their configured default;
  - pages use **opaque continuation tokens**, so an adapter maps them onto whatever its provider offers (Jellyfin's offset pagination stays behind the adapter);
  - TV seasons and episodes are outside contract v1; adding them later is additive;
  - an item's content metadata travels inside an embedded `TitleMetadata` (`metadata`), including title and year — they are matching evidence like any other field, and which fields drive a merge is decided by the core's matching engine, never by the schema (§1.24 records the removal of the former top-level copies);
  - items removed upstream are not deleted from the cache in M1; deletion reconciliation arrives with the identity work (M2).
- **Rationale:** the catalogue index is the browse surface of a library-class provider (§3.2 of PLAN.md); naming it once here keeps adapters, fixtures, and the registry consistent.
- **Consequence:** adapters declare `meta` + `browse` v1 and nothing more until further suites ship; the provider fixture suite (`fixtures/provider/v1/`) is the conformance gate for all of the above.

### 1.22 Source-cache refresh mechanism

- **Decision:** each linked account's catalogue re-syncs on the shipped default cadence (§1.14: **6 h**), overridable per slot via `sync-cadence`. Syncs are spread by **±10 % jitter** of the wait so accounts never fire in lockstep; a failing sync backs off exponentially starting at **1 minute**, capped at **24 hours**, resetting to the normal cadence after a success.
- **Cadence precedence:** an adapter MAY declare its own polite default in its handshake response, under the `policy` map added to `CapabilityQueryResponse` (additive; e.g. Jellyfin declares `browse.sync-cadence: 6h`). Resolution order: explicit operator config wins over the adapter-declared value, which wins over the shipped scheduler default. An explicit config value or adapter declaration that fails to parse is a startup error, never silently ignored. (Representation of the declaration channel itself: §1.23.)
- **Rationale:** a self-hosted instance must tolerate a provider being down without spinning or giving up; jitter keeps many accounts from hammering providers simultaneously. Letting adapters state their own default keeps new slots well-behaved with zero operator config, while the config override keeps the operator sovereign.
- **Consequence:** these are the values `core/internal/scheduler` runs; changing them is a decision-record change, not a code-taste question. The aggregate multi-provider governor remains deferred to its planned milestone (§1.14).

### 1.23 Handshake operating-policy representation

- **Decision:** slot-declared operating policy travels in the `policy` map on `CapabilityQueryResponse` — a plain `map<string,string>` added to the frozen v1 contract. Each key is declared by an adapter and interpreted only by that adapter's wiring; the core stores the map opaquely and ignores unknown keys, so adapters can grow their vocabulary without touching the contract.
- **Promotion path (deliberately kept open):** if a key ever proves *universally meaningful across providers* rather than common within one, promote it to a typed field on the response. Promotion is an ordinary additive change (declare the typed field, deprecate the map key, remove once no slot sends it); going back — typed → map — would be breaking. That asymmetry is why typing starts deferred: it can be adopted later at low cost, but locking vocabulary in early cannot be undone cheaply.
- **Rationale:** exactly one policy exists today (§1.22's sync cadence). Typing every future key up front turns adapter evolution into contract ceremony and accumulates fields whose validity depends on which capability a slot speaks; the map ties each key to whoever declares it.
- **Consequence:** well-known key names live beside their adapters (wiring + decision entries), never in a shared registry; the core must never branch on key names — the moment it wants to is the moment to revisit this entry and promote the key instead.

### 1.24 Pre-release contract-evolution policy — **breaking allowed, gate off**

- **Decision:** until the first release ships (SCOPE.md), built-in slot contracts may evolve **breaking-ly without a version bump**: every consumer is in-repo and is updated atomically in the same change, so version-bump ceremony (new suite, new handshake, per-break log entries) buys nothing while no external consumers exist. The `buf breaking` stage is disabled for this period — commented out of the Makefile's lint recipe rather than scoped per package. The exemption ends at first release: before any contract is published, the gate returns and PLAN.md's versioning rule applies without exception.
- **First use:** the whole-catalogue sync item (`CatalogueItem`) dropped its top-level `title`/`year` fields; all content metadata — including those two matching-evidence values — now travels inside an embedded `TitleMetadata` (`metadata`). Which fields drive a merge is decided by the core's matching engine, never by the schema.
- **First use:** `CoverageRow.via` became **repeated**: one coverage claim can be observed by several linked accounts of one provider slot, and last-writer-wins attribution silently lost that fact. Each element keeps the documented `account:provider:host` form; derivations sort elements so rebuilds are deterministic.
- **Rationale:** single-operator pre-release project; the load-bearing freeze exists to protect external consumers that do not yet exist.
- **Consequence:** load-bearing (`core/v1`) and API (`api/v1`) schemas lose automated drift detection too, not just slot contracts. Compensating controls: fixture suites and round-trip tests catch shape changes from below, review discipline catches them from above (AGENTS.md protected-file process). A re-enabled gate must be paired with a published baseline tag.

### 1.25 Provider namespace — **the slot instance id**

- **Decision:** everything keyed by "provider" — source-cache keys, provider-item-registry mappings (`(provider, nativeId) → {entryId, proof}`), coverage keys, availability- and merge-conflict-event payloads — uses the **slot instance id** (`SlotEntry.id`, e.g. `home-jellyfin`), never the adapter name. An adapter name cannot disambiguate two deployed instances of one adapter (two separate Jellyfin servers numbering their items identically); the slot id scopes identity per deployment while accounts of one slot share it deliberately (same server ⇒ same items ⇒ shared identity work).
- **Rationale:** PLAN.md §2.3's scoped provider item ID (`provider:nativeId`) needs `provider` to mean what the operator actually deployed; §5.3's coverage map keys its rows by providerId for the same reason.
- **Consequence:** renaming a slot id in config re-keys identity state (fresh resolutions; old aliases stay resolvable but unused) — treat slot ids as stable identifiers. Recorded alongside the M2 identity work.

### 1.26 Merge-conflict events — **emission proven now, delivery deferred to M6**

- **Decision:** the provider item registry detects recycled native IDs and proof divergences and builds OWNER-audience merge-conflict envelopes whenever constructed with an owner id; the M2 milestone tests prove that end to end (M2 acceptance "merge-conflict events"). Production composition deliberately constructs the registry **without** an owner id, so a running instance emits none until the operator surface exists. There is no owner concept before sharing lands (M6), and an OWNER-audience event needs a concrete owner user id for tenancy-routed delivery — inventing owner semantics earlier would be a product decision taken silently in wiring code.
- **The suppressed path is three gates**, named here so the deferral reads as one decision, not three oversights: (1) the empty owner id suppresses emission at the source; (2) the sync path cannot carry envelopes — the source-cache resolver seam returns only an error and the slot-wiring adapter discards the registry's returned events; (3) the event mux forwards availability payloads only. Enabling delivery therefore means owner semantics, a resolver-seam extension, and a mux routing rule together — not a one-liner.
- **Rationale:** PLAN.md's safety half holds unconditionally — conflicting identities never merge silently, entries stay apart, and the registry keeps both mappings durably. Only the report waits; the event bus is ephemeral by design, so events emitted today could not be replayed later anyway.
- **Consequence:** until M6, a divergence is observable only in stored state (both registry mappings), never via events or UI. M2 closes on its milestone tests proving emission capability; the delivery half rides with M6's account-scoped event routing and owner roles.

### 1.27 First catalogue provider — **TMDB**

- **Decision:** M3's catalogue enrichment adopts TMDB as the first real catalogue adapter (the built-in reference slot stays the offline conformance baseline). Free API, bearer-token auth, movie+series coverage, and IMDb cross-links in both directions (inline `imdb_id` out, `/find` in) make it the lowest-friction fit for PLAN.md §5.2's catalogue-preferred rule.
- **Endpoint mapping (evidence: RESEARCH.md §2.4):** `LookupTitle` → `/search/movie|tv`; `GetMetadata` accepts *any* `namespace:value` external ID — unknown-to-TMDB namespaces resolve via `/find` (IMDb), known ones pass through; details fetched once with `append_to_response=external_ids,credits,content-ratings` (single round trip, no N+1). Records key on `tmdb:{id}`; `imdb:`/`wikidata:` IDs become aliases linked in the metadata cache.
- **Concrete values:** posters stored as full URLs at size `w500`; client pacing defaults to well under the soft limit (serial worker, ~5 req/s ceiling, back off on `429` per `Retry-After`); language fixed to `en-US` for v1.
- **Credential delivery (interim):** the bearer token is a slot-level instance secret delivered like provider passwords: `SlotEntry` gains an optional `token-env` naming the environment variable read at composition time (test name documented in `.env.example`). Values never live in config files. This migrates behind an operator surface when one exists.
- **Consequence:** fixtures exercise the catalogue contract against a canned adapter, never the live API; live behavior is verified once, manually, when the adapter lands.

### 1.28 Enrichment execution model — **triggers enqueue, one paced worker drains**

- **Decision:** all enrichment runs through one background work queue behind a narrow Go seam — producer side `Enqueue(entryID)` (idempotent: an entry already pending is coalesced, not duplicated), consumer side drained by a single paced worker registered as a scheduler job. The seam mirrors the event-bus pattern: small interface, in-memory implementation first, injected at composition time — no config knob until a second backend exists. Producers never enrich inline: T1 marks entries whose `metadata_ref` derivation missed during derived-library rebuilds and enqueues them; T2 enqueues entries whose sync introduced new identity evidence. Future trigger kinds (operator-initiated refresh, staleness sweeps) plug in as additional producers without touching engine or queue (IMPLEMENTATION.md §4.1 growth rule).
- **Queue backends:** in-memory first, same posture as the event bus. The interface keeps heavier transports open — durable SQLite, Kafka, or whatever scale demands — as pure implementation swaps: producers, engine, and worker never learn which backend runs. Durability stays optional by design: T1 re-marks misses on every rebuild and T2 re-enqueues on every sync, so a lost queue self-heals regardless of backend.
- **Resolution protocol per entry:** external IDs first — `GetMetadata` resolves any identity assertion through the catalogue contract regardless of namespace; text `LookupTitle` is the fallback for entries without usable IDs. Returned candidates are scored against the entry's own full evidence by the matching engine — summaries first, details fetched only for genuine near-ties — and the corroboration gate enriches on exactly one survivor.
- **Ambiguity abstains:** ties after full scoring leave the entry un-enriched, logged, with no marker persisted — never guess stays absolute (PLAN.md §5.3). v1 deliberately skips review-marker state; `GetMetadata(ref)` already leaves room for a manual-pick surface later without contract change.
- **Config surface:** catalogue slots are enabled by listing them under `slots.catalogue`; the drain cadence overrides through the existing declared-cadence precedence chain. No speculative knobs beyond enablement + cadence. The queue itself is in-memory state rebuilt from marked misses, so a crashed worker loses nothing durable.
- **Rationale:** matches the converged pattern across Jellyfin's priority queue, Emby's scheduled tasks, Plex's maintenance window, and Radarr's non-disableable refresh task — background, paced, never in the request path.

## 2. Open implementation items (recorded, not decided)

These are deferred by design or pending follow-on choices:

- **Subprocess slot supervision mechanics** — per-slot transport config (§3.1 of PLAN.md); decided when the first subprocess slot ships. v1 ships **no subprocess slots**: all built-in adapters are in-process (§1.1); the Jellyfin adapter is in-process Go making outbound HTTP calls.
- **Store backends** — resolved for v1: SQLite (§1.16); non-SQLite backends remain possible per class at implementation time.
- **Version-pin values** — resolved: initial set in §1.4, re-verified at M0 scaffolding.
- **Merge-conflict event delivery** — deferred to M6 alongside owner semantics and account-scoped routing (§1.26); emission capability exists and is test-proven.

## 3. Relationship to the other documents

- **PLAN.md** — the spec these choices realize; product decisions live in its §11.
- **SCOPE.md** — which milestones ship in v1; this document never overrides scope.
- **RESEARCH.md** — evidence for feasibility choices (e.g., §1.8) before they are decided here.
- **ENVIRONMENT.md** — defines reproducibility; this document supplies the concrete pins and layout it stays agnostic about.
- **IMPLEMENTATION.md / TESTING.md / CI-CD.md** — consume the language, transport, and pins recorded here.
