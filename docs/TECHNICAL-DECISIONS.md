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

### 1.3 Repository layout

- **Decision:**

```text
/
├── proto/                # .proto schemas — single source of truth (§2.1 of PLAN.md)
│   ├── abcmovies/core/v1/     # load-bearing contracts (PLAN.md §2.3)
│   ├── abcmovies/slots/v1/    # slot kinds (PLAN.md §3.1)
│   └── abcmovies/api/v1/      # inbound API service surface (§8)
├── core/                 # Go module: the core service
├── adapters/             # built-in slot implementations (Go, one dir per adapter)
├── frontends/            # frontend clients (web, CLI)
├── fixtures/             # fixture suites: fixtures/<contract>/v<version>/ (+ handshake/, negative/)
├── tests/                # milestone tests, mirrors M0–M9
├── docs/                 # PLAN.md, IMPLEMENTATION.md, ENVIRONMENT.md, TESTING.md, CI-CD.md,
│                         # TECHNICAL-DECISIONS.md, SCOPE.md, RESEARCH.md, THREAT-MODEL.md, OPERATIONS.md
├── Containerfile         # dev + CI image (ENVIRONMENT.md §3)
├── Makefile              # make deps / proto / fmt / secret-scan / lint / build / test-unit / vuln / check / run
├── .tool-versions        # canonical version pins (ENVIRONMENT.md §1)
├── package.json          # formatting/linting tooling pins: prettier, markdownlint-cli2 (§1.4)
├── .golangci.yml         # golangci-lint v2 config — gofumpt formatting enforcement
├── .prettierrc           # prettier style (JSON, YAML) and targets (.prettierignore)
├── .markdownlint-cli2.yaml        # markdownlint style and targets (Markdown)
├── config.example.yaml   # committed example of the instance config
├── .env.example          # committed names of local test credential variables
└── LICENSE               # AGPL-3.0
```

- **Rationale:** mirrors the plan's own separations — contracts (proto/), core, slots (adapters/), clients (frontends/), conformance (fixtures/), milestones (tests/). Proto directories mirror the package paths (`abcmovies.<kind>.v1`), so the schema linter's package-directory-match rule holds with no exceptions (§1.18).

### 1.4 Version pins

- **Decision:** pins live at **one home per tool kind**, never duplicated: the core language pins itself via its manifest's `toolchain` directive (auto-enforced on every tool invocation), and the schema tooling + linter live in `.tool-versions` at the repo root, referenced by the Containerfile and CI (ENVIRONMENT.md §1, CI-CD.md §3). A change to a pin invalidates caches (CI-CD.md §8). Go is not listed in `.tool-versions` because the Go toolchain cannot be version-switched on hosts where developers already manage Go; the manifest's `toolchain` directive (GOTOOLCHAIN=auto) makes the manifest authoritative anyway, so listing Go twice would reintroduce the exact split-brain this rule prevents.
- **Initial pin set** (re-verified at M0 scaffolding, IMPLEMENTATION.md §8.4): Go **`go 1.26` / `toolchain go1.26.6`** in `core/go.mod`; **buf 1.72.0**, **golangci-lint v2.12.2**, **node 24.19.0** (runtime for the formatting tooling), and **gitleaks 8.24.3** (secret-leak scan) in `.tool-versions`; as Go tool dependencies in `core/go.mod` (executables installed to `bin/` by `make deps`, §1.6): protoc-gen-go **v1.36.12** (always equal to the `google.golang.org/protobuf` runtime version), protoc-gen-go-grpc **v1.6.2**, grpc-go **v1.83.0**, and **govulncheck v1.7.0** (vulnerability scan, `make vuln`). The node-based formatters are npm dev dependencies (`prettier`, `markdownlint-cli2`), pinned in `package-lock.json` — their versions live there, not in `.tool-versions`.

### 1.5 Instance config location and secrets convention

- **Decision:** the instance reads configuration from a single YAML file at `config/instance.yaml` (gitignored), with a committed example at `config.example.yaml` and variable *names* documented in `.env.example` (ENVIRONMENT.md §6, CI-CD.md §4). Values never enter the repository.
- The vault's encryption keys and the host relay key (§7.6 of PLAN.md) are generated locally, never committed, never logged.

### 1.6 Make recipes

- **Decision:** the Makefile is the single entry point, matching ENVIRONMENT.md §5 and CI-CD.md §1 exactly — CI never invents its own commands.

| Recipe | Runs |
| --- | --- |
| `make deps` | language-level dependencies from the pinned versions (Go tools, `npm ci`) |
| `make proto` | regenerate code from the `.proto` schemas |
| `make fmt` | rewrite formatting in place — `buf format`, gofumpt, prettier |
| `make check` | lint + build + full suite (unit, round-trip, integration, fixture suites) + vuln — the CI gate |
| `make run` | boot the skeleton (registry, slot, API server) |
| `make lint` / `make build` / `make test-unit` / `make vuln` | the individual stages `make check` composes |

`make lint` enforces all formatting and hygiene mechanically: buf lint + format freshness on the schemas, gofumpt on Go, prettier on JSON/YAML, markdownlint on Markdown, and the secret-leak scan (`make secret-scan`). `make test-unit` runs the unit suites with the race detector. `make vuln` runs govulncheck against the module. CI runs these recipes verbatim (CI-CD.md §1); it never invents its own commands.

### 1.7 CI platform — **GitHub Actions**

- **Decision:** GitHub Actions. The worked example that CI-CD.md §7 used to carry is recorded here so CI-CD.md stays vendor-neutral; the pipeline stages and gates are vendor-neutral and the platform is interchangeable.
- **Stage mapping:**
  - **lint, typecheck/build** — a job per stage on a fresh runner from the pinned toolchain (the same Containerfile as ENVIRONMENT.md §3). The lint job also runs the secret-leak scan (`make secret-scan`) — the "CI secret-leak gate" THREAT-MODEL.md T12 and CI-CD.md §4 promise.
  - **schema checks** — a dedicated job running the schema lint and breaking-change tooling; fail on breaking change without a version bump.
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

- **Decision:** the contracts (§2.1 of PLAN.md) are encoded as Protobuf schemas; the `.proto` definitions are the single source of truth. Codegen and schema checks run through the **buf** toolchain: `buf generate` (invoking the pinned per-language plugins below), `buf lint`, `buf breaking` (against the last approved state — satisfies the schema gate in CI-CD.md §3), and `buf format`.
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
- **Consequence:** the sessions store holds the token's hash, never the token value; a token is minted at login and revoked per session. Delivery-session liveness is separate: heartbeat and TTL (§9.1 of PLAN.md, defaults in §1.14).

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

### 1.18 Schema lint and breaking config — **buf, no exceptions**

- **Decision:** `buf lint` runs the **STANDARD** rule set and `buf breaking` runs the **FILE** rule set, with no disabled rules. The proto directory tree mirrors package paths (`abcmovies.<kind>.v1`, §1.3), so the lint rule that demands a file's directory match its package name passes by construction.
- **Rationale:** a schema gate without exceptions (CI-CD.md §3) needs lint config that a contributor cannot silently widen; mirroring directories to packages removes the one rule that would otherwise need a carve-out.
- **Consequence:** the proto files' package name and their directory path are locked together; moving either one is a schema change (IMPLEMENTATION.md §8.1).

### 1.19 Fixture suite format — **JSON descriptor + JSON cases**

- **Decision:** a fixture suite (TESTING.md §3) is a JSON directory under `fixtures/<contract>/v<version>/` with:
  - `suite.json` — a descriptor: `{ "contract", "version", "kind" (one of handshake | positive | negative), "slot" (the slot under test), "transport" (defaults to the slot's transport) }`.
  - `cases/*.json` — one file per case, each a sample request with its expected response for `handshake`/`positive` kinds, or an invalid declaration the registry must reject for `negative` kind.
- **Rationale:** JSON is a language-neutral data format (the conformance gate is language-neutral, TESTING.md §2.1); one case per file keeps negative fixtures (mandatory for anything that accepts input) readable and diffable.
- **Consequence:** the format is data, not code — any adapter in any language can run a suite; the generic runner (`core/cmd/fixture-runner`, TESTING.md §3) consumes it.

## 2. Open implementation items (recorded, not decided)

These are deferred by design or pending follow-on choices:

- **Subprocess slot supervision mechanics** — per-slot transport config (§3.1 of PLAN.md); decided when the first subprocess slot ships. v1 ships **no subprocess slots**: all built-in adapters are in-process (§1.1); the Jellyfin adapter is in-process Go making outbound HTTP calls.
- **Store backends** — resolved for v1: SQLite (§1.16); non-SQLite backends remain possible per class at implementation time.
- **Version-pin values** — resolved: initial set in §1.4, re-verified at M0 scaffolding.

## 3. Relationship to the other documents

- **PLAN.md** — the spec these choices realize; product decisions live in its §11.
- **SCOPE.md** — which milestones ship in v1; this document never overrides scope.
- **RESEARCH.md** — evidence for feasibility choices (e.g., §1.8) before they are decided here.
- **ENVIRONMENT.md** — defines reproducibility; this document supplies the concrete pins and layout it stays agnostic about.
- **IMPLEMENTATION.md / TESTING.md / CI-CD.md** — consume the language, transport, and pins recorded here.
