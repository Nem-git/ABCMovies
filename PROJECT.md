# ABCMovies — Project Document

## Vision

ABCMovies is a self-hosted media server that aggregates streaming services into a
single catalog and a proxied playback experience. Sources are pluggable — web
streaming services, existing CLI tools (unshackle, devine), local files, and
databases. The core orchestrates metadata, DRM, transcoding, and downloads behind
stable, swappable interfaces.

ABCMovies is inspired by — and designed to interoperate with — the ecosystem of
existing streaming/DRM projects: rosso, diana, CDRM-Project, Unshackle-Services,
pyplayready, pywidevine, VT-PR, freevine, and N_m3u8DL-RE. Rather than reimplement
any of them, ABCMovies defines interfaces around them and lets each be plugged in.

## Guiding principles

1. **Pluggable everything.** No blessed implementations, no blessed protocols, no
   blessed frontends. The core defines interfaces; implementations are selected by
   configuration.

2. **Symmetric adapter model.** The core is a neutral bus of **canonical
   operations**. Two adapter farms surround it:
   - **Frontend adapters** — how clients interact with the core (REST, JSON-RPC,
     CLI, WebSocket, gRPC, ...). New interface = new adapter; every adapter exposes
     the *same* canonical operations.
   - **Backend adapters** — how the core reaches sources and engines (subprocess,
     stdio JSON-RPC, HTTP, filesystem/DB, in-process, ...). New mechanism = new
     adapter.

3. **A service, not an application.** ABCMovies is a headless capability hub. The
   web UI, REST API, and CLI are optional *frontends*, not the product. It can run
   with zero frontends and be driven purely over JSON-RPC, or behind any custom
   frontend. The product is the service bus plus its adapter farms.

4. **Interface-first.** Delivery mechanisms, download engines, CDM backends, and
   frontends are defined as contracts now and plugged in later.

5. **Roles × mechanisms.** Every integration is described by two independent axes:
   *what it provides* (role) and *how it works* (mechanism). This keeps "an HTTP
   source plugin" and "ffmpeg" as clearly distinct things, while still letting the
   core reason about them uniformly.

## High-level architecture

```
 Frontend adapters (any protocol)          Backend adapters (any mechanism)
 ┌────────────────────────────┐   core   ┌──────────────────────────────┐
 │ REST API      (OpenAPI)    │ ┌──────┐ │ Source    · subprocess / cli │
 │ JSON-RPC      (any methods)│ │Core  │ │ Source    · stdio JSON-RPC   │
 │ CLI (direct / remote)      │ │server│ │ Source    · HTTP             │
 │ WebSocket     (future)     │ │bus   │ │ Source    · filesystem / db  │
 │ gRPC          (future)     │ │ops   │ │ Metadata  · any              │
 │ Web UI (HTMX) = uses REST  │ └──────┘ │ Devices   · local/user/remote│
 └────────────────────────────┘   ^       │ Keys      · pywidevine/...  │
        SQLite │ Vault │ Registry │ Job Queue ── │ Decrypt · transcode │
                                                   │ download engines    │
                                                   └─────────────────────┘
```

- **Canonical operations** are declared once, implemented once. Every frontend
  adapter auto-exposes them: REST as `POST /api/v1/search`, JSON-RPC as
  `catalog.search`, CLI as `abcmovies search`. Adding a feature = add one
  operation; all frontends get it. Adding a frontend = one adapter; all operations
  appear.
- **Web UI** is a reference frontend (HTMX + hls.js) sitting on the REST gateway —
  not a blessed one. Third-party frontends may use JSON-RPC, REST, or any adapter.

## Domain model

- **Title** — a movie, series, or channel. Holds a canonical id, kind, name, year,
  genres, `ExternalIDs` (imdb / tmdb / tvdb), and seasons + episodes for series.
- **Track** — a video/audio/subtitle stream: codec, resolution, bitrate, language,
  channels, color range (SDR / HDR10 / DV), forced and SDH flags.
- **Stream** — a playable descriptor: protocol (HLS / DASH / ISM / progressive),
  source URLs, `IsLive` flag, `DRMInfo` (system, PSSH, license URL and headers),
  and available tracks.
- **Key** — a content key: `kid:key` plus DRM system.
- **CDMDevice** — a Widevine (`.wvd`) or PlayReady (`.prd`) device blob, plus
  metadata (system, origin, owner).
- **Account** — a user's credentials for a streaming service (encrypted at rest).
- **Grant** — explicit owner → user access to a specific account.
- **Job** — a download or long-running task request.

## Canonical operations (the service bus)

The core exposes a single operation set that every frontend adapter maps onto:

- **Auth** — login, logout, me
- **Catalog** — search, catalog/browse, get title, get metadata
- **Playback** — get tracks, resolve, stream session lifecycle
- **Accounts** — user accounts, grants (share / revoke), host shared-account pools
- **CDM Vault** — list / upload devices, per-title CDM selection
- **Jobs** — create, list, status, cancel (downloads, tasks)
- **Admin** — plugins, config, health, instance status

## Core interfaces

### SourcePlugin — role: Source

A source may implement a subset of these capabilities:

```go
type SourcePlugin interface {
    Search(ctx context.Context, query string) ([]Title, error)
    Catalog(ctx context.Context, filter CatalogFilter) ([]Title, error)
    GetMetadata(ctx context.Context, ref string) (*Title, error)
    GetTracks(ctx context.Context, ref string) ([]Track, error)
    Resolve(ctx context.Context, ref string, opts ResolveOpts) ([]Stream, error)
}
```

### MetadataProvider — role: Metadata

External enrichment (TMDB, IMDb, TVDB), matched to a title via `ExternalIDs`:

```go
type MetadataProvider interface {
    Enrich(ctx context.Context, title *Title) (*Title, error)
}
```

### DeviceSource — role: Devices

Where CDM devices come from. Deliberately open — a deployment may use local files,
user uploads, or a third-party CDM API:

```go
type DeviceSource interface {
    List(ctx context.Context, filter DeviceFilter) ([]CDMDevice, error)
    Get(ctx context.Context, id string) (*CDMDevice, error)
    // upload/store is optional per source
}
```

### KeyProvider — role: Keys

Given a stream's license info and a device, produce content keys:

```go
type KeyProvider interface {
    GetKeys(ctx context.Context, stream *Stream, device *CDMDevice) ([]Key, error)
}
```

### Decryptor — role: Decrypt

```go
type Decryptor interface {
    Decrypt(ctx context.Context, in *Stream, keys []Key) (*Stream, error)
}
```

### Transcoder — role: Transcode

Clear stream + target profile → delivered output. Handles both transmuxing and
encoding (see pipeline).

```go
type Transcoder interface {
    Process(ctx context.Context, in *Stream, profile Profile) (*Output, error)
}
```

### Downloader — role: Download

```go
type Downloader interface {
    Download(ctx context.Context, job *Job) error
}
```

### TransportAdapter — backend mechanism

Translates canonical `SourcePlugin` calls to a plugin's chosen protocol
(subprocess / stdio JSON-RPC / HTTP / filesystem / DB).

## Backend adapters: roles × mechanisms

Every integration is a pair of two independent axes.

| Role (what it provides) | Mechanism (how it works) |
|---|---|
| Source     | Subprocess / CLI (run binary/script, parse output) |
| Metadata   | stdio JSON-RPC (plugin subprocess) |
| Devices    | HTTP(S) (call a remote service/API) |
| Keys       | Filesystem / DB (read files, DBs, `.wvd`/`.prd`) |
| Decrypt    | In-process (Go library compiled in) |
| Transcode  | remote gRPC / WS (future) |
| Download   | |

Examples of concrete pairings:

- **ffmpeg** = *Transcode* × *Subprocess/CLI*
- **N_m3u8DL-RE** = *Download* × *Subprocess/CLI*
- **unshackle / devine wrapper** = *Source* × *Subprocess/CLI*
- **streaming-service plugin** = *Source* × *HTTP* or *Source* × *stdio JSON-RPC*
- **local-files source** = *Source* × *Filesystem/DB*
- **native Go downloader** = *Download* × *In-process*

"HTTP" is not a kind of adapter — it is a mechanism that a *Source* or *Devices*
adapter may use. ffmpeg is not a transport — it is a *role* implemented via
subprocess. Roles are typed and discoverable; mechanisms are an implementation
detail of each adapter.

## CDM management

CDM handling is split into two pluggable roles so nothing about it is hardcoded:

- **Devices** — where CDM devices come from:
  - `local` — read `.wvd` / `.prd` from disk (host pool)
  - `user` — devices uploaded by users
  - `remote-api` — reach a third-party CDM API for devices/keys (no local device
    required)
- **Keys** — given a device, turn a license request into content keys:
  - `pywidevine` (Python sidecar), `pyplayready`, native Go CDM (e.g. built on
    diana), etc.

Per-title CDM selection: the requesting user may choose a specific device from
their devices or the host pool; otherwise a configured default applies. A
deployment could run entirely on remote CDM APIs with zero local devices.

## Registry

The registry is the core's phonebook. Every adapter announces itself on load:

```go
reg.Register(source.Source{Name: "tubi", ...})          // role: source
reg.Register(keys.KeyProvider{Name: "pywidevine", ...}) // role: keys
reg.Register(transcode.Transcoder{Name: "ffmpeg", ...}) // role: transcode
reg.Register(devices.DeviceSource{Name: "local", ...})  // role: devices
```

Lookups are by role and name; the rest of the core never hardcodes a tool:

```go
reg.Get("source",     "tubi")         // search/catalog/etc.
reg.Get("metadata",   "tmdb")
reg.Get("devices",    "local")        // or: "user", "remote-api"
reg.Get("keys",       "pywidevine")   // or: "diana", "pyplayready"
reg.Get("decrypt",    "mp4decrypt")
reg.Get("transcode",  "ffmpeg")
reg.Get("download",   "n_m3u8dl-re")
```

Config decides the names (`transcoder: ffmpeg`); the registry returns whatever is
registered under that name. This is what makes "pluggable everything" real: swap
one line of config and the whole pipeline changes behavior.

## Proxy pipeline

A composable chain of stages:

```
Resolve → [Keys + Devices] → [Decrypt] → [Transcode/Transmux] → Deliver
```

- Each stage is optional and configurable per source and per stream.
- Clear streams skip the key/decrypt stages.
- Transcode profile comes from the request (output format, codec, resolution,
  container) or from instance defaults.
- Live channels are streams flagged `IsLive`, re-streamed through the same
  pipeline.

## Accounts & sharing

- Users link their own streaming-service credentials per source (credential vault,
  encrypted at rest).
- **Grants** — an owner grants specific users access to a specific account, and
  can revoke it.
- **Host shared accounts** — accounts registered in the instance config, usable by
  all users.
- Session locking for shared accounts is deferred (future).

## Authentication

- Local accounts with registration on/off (admin creates users).
- Session cookies for web frontends.
- Bearer tokens for REST / JSON-RPC / CLI clients.

## Persistence & configuration

- **SQLite** — embedded, for users, accounts, grants, jobs, device index, cache.
- **`config.yaml`** — instance config: plugin directory, device pools, shared
  accounts, metadata provider keys, transcoder profiles, gateway enable/disable.

## Frontends (interaction surfaces)

- **REST API** — OpenAPI 3.1 spec, generated Go server; the reference gateway.
- **JSON-RPC** — methods mapped onto the canonical operations
  (`catalog.search`, `title.streams`, ...).
- **CLI** — Go binary; runs against the REST/JSON-RPC gateway remotely, or
  in-process for local/admin use.
- **Web UI** — HTMX server-rendered pages + hls.js player; uses the REST gateway.
- **Future** — WebSocket, gRPC, or anything else, as new adapters.

## Project layout (greenfield monorepo)

```
cmd/abcmovies/          server binary
cmd/abcmovies-cli/      CLI (remote or in-process)
internal/ops/           canonical operations (the service bus)
internal/gateway/       frontend adapters: rest, jsonrpc, cli
internal/adapters/      backend adapters: subprocess, jsonrpc, http, local
internal/models/        domain types
internal/api/           REST handlers
internal/web/           HTMX handlers + templates
internal/registry/      capability registry
internal/pipeline/      proxy engine
internal/vault/         credentials + CDM device sources
internal/jobs/          job queue
internal/store/         SQLite
internal/auth/          sessions/tokens
plugins/                shipped example source plugins
api/openapi.yaml        OpenAPI for the REST gateway
PROJECT.md
```

## Roadmap

1. **Foundation** — core server, SQLite store, registry, canonical ops, auth,
   config, REST gateway + OpenAPI.
2. **First sources** — local-files source (Filesystem/DB) and one HTTP source
   plugin; catalog, search, metadata (TMDB enrichment).
3. **Pipeline** — resolve → (decrypt) → (transcode) → delivery for clear content;
   HLS output; hls.js playback in the web UI.
4. **CDM layer** — devices (local + user) and keys (pywidevine sidecar);
   per-title device selection.
5. **Accounts & sharing** — credentials, grants, host shared-account pools.
6. **Downloads** — job queue + N_m3u8DL-RE downloader; track selection
   (quality/codec/audio/subs/format/muxer).
7. **More frontends** — JSON-RPC gateway, CLI commands, WS/gRPC.
8. **Live** — live-channel sources and re-streaming through the pipeline.

## Open / deferred (by design)

- Concrete delivery mechanisms (on-demand HLS vs full-file cache) — interface only.
- Concrete downloader implementations — all engines behind `Downloader`.
- CDM backends (Python sidecar vs Go-native) — both behind `Keys` + `Devices`.
- Session locking for shared accounts.
- Live channel guide / EPG details.

## Disclaimers

- DRM decryption requires CDM devices supplied by the operator or users; ABCMovies
  does not bundle them.
- Users are responsible for lawful use of their own accounts and content.
- ABCMovies does not condone piracy; it is an orchestration layer around the
  existing open-source ecosystem.
