# Roadmap

Feature ideas inspired by an analysis of DRM/streaming archival projects
(Devine, UnShackle, VineTrimmer, 3052/diana, 3052/rosso) and how they map onto
ABCMovies' existing architecture. The reference projects were studied for their
configuration handling, feature set, and architecture — not their decryption
internals.

## Cool projects:
- https://github.com/3052/rosso
- https://github.com/TPD94/CDRM-Project-2.0
- https://git.drmlab.io/kufei/Unshackle-Services
- https://git.gay/ready-dl/pyplayready
- https://github.com/devine-dl/pywidevine
- https://git.drmlab.io/ayuspie/VT-PR
- https://github.com/3052/diana
- https://github.com/stabbedbybrick/freevine
- https://github.com/nilaoda/N_m3u8DL-RE

## Reference projects — best / worst at a glance

| Project | Best | Worst |
|---|---|---|
| **Devine** (`devine-dl/devine`) | Service-tag plugin framework, layered config, multi-profile auth | Ships zero services; PyPI release yanked; thin docs |
| **UnShackle** (`unshackle-dl/unshackle`) | Config precedence chain, REST API (job queue + per-key auth), gold-standard config reference | Config sprawl, huge surface, roadmap churn |
| **VineTrimmer** (`xzork11/VineTrimmer`) | Preview flags (`--list`/`--keys`), per-service CLI groups, wanted-episode syntax | Windows-first, monolithic services, aging |
| **3052/diana** | Library-first, semver, pure-Go Widevine/PlayReady | App-level features absent; OSL-3.0 license |
| **3052/rosso** | Provider/catalog metadata model | App-level features absent; PolyForm-Noncommercial license |

---

## Tier 1 — small effort, high value

- [ ] **Persistent key vault** (Devine/UnShackle)
  Add a SQLite-backed `VaultStore` so licensed keys survive restarts. Drop-in
  behind the existing `VaultStore` interface (`internal/drm/vault.go`) which
  already has `Get/Put/expiresAt`. Also enables a `/keys` listing for debugging.
  Config: `drm.vault.type: memory | sqlite`.

- [ ] **Per-provider upstream proxy** (UnShackle proxy providers)
  Add `proxy.http_proxy` to `ServiceEntry` and set `Transport.Proxy` on the
  fetcher (`internal/proxy/fetch.go`) via `http.ProxyURL`. Enables
  geo-specific providers (CBC/TOU are CA-only today).

- [ ] **API key auth + per-key service allowlists** (UnShackle `serve`)
  `ApiSecurity` middleware (`internal/middleware/security.go`) currently sets
  CORS only — no auth. Add an optional static key / per-key `services:`
  allowlist mirroring UnShackle's model. Foundation for the automation API.

- [ ] **Swagger UI on the ogen server** (UnShackle `/api/docs`)
  Ogen serves OpenAPI docs nearly free; `api/openapi.yaml` already exists.
  Useful for automation and the API work below.

## Tier 2 — moderate, directly on the roadmap

- [ ] **DASH SegmentBase / BaseURL support** (`TODO.md` already lists this)
  `convert/mp4/dash.go` returns "not supported" for SegmentBase. Implement
  BaseURL chain resolution (RFC 3986) + Range-aware serving.

- [ ] **Smooth Streaming (ISM) parsing** (UnShackle, VineTrimmer-PlayReady)
  `TODO.md` notes "Add Smooth support" for rcmedia. This is the format leap
  UnShackle made over Devine.

- [ ] **HTTP/remote key service** (UnShackle `remote_cdm`, key vaults)
  Beyond SQLite: expose a license/key HTTP API so a client can decrypt without
  holding device material. Aligns with the planned browser-playback license
  proxy (`DRM.md`).

- [ ] **Multi-profile auth per provider** (Devine/VineTrimmer profiles)
  TOU creds are currently a single env pair (`README.md`). Add a `profiles:`
  list per service (cookies or credentials), selected per request. Needed for
  per-region/account rotation.

- [ ] **Track-preview / selection as API params** (VineTrimmer `--list`,
  `--keys`, `--wanted`)
  Add `?quality=1080&range=SDR`-style filters to
  `GetMovieStreams`/`GetEpisodeStreams` and a "keys only" debug mode.

## Tier 3 — bigger, more opinionated

- [ ] **Config precedence system** (UnShackle)
  Formalize `CLI > env > per-provider override > global > default`. Today env
  only overrides `BASE_URL`/`API_PREFIX` (`internal/config/config.go`). Add
  per-provider defaults (page size, proxy, quality) and a `cfg`-style
  subcommand or admin endpoint to read/set config live.

- [ ] **Job queue + progress/history API** (UnShackle `serve`)
  The convert pipeline is a synchronous HTTP stream
  (`convertedStreamFile` in `internal/handler/handler.go`). A queue with
  progress events would let long transmuxes be polled/notified instead of
  blocking. Pairs naturally with the automation API.

- [ ] **Subtitle/chapter merging into MP4/MKV** (Devine/UnShackle muxing)
  `writeFMP4Merge` drops text adaptation sets (`isTextAdaptationSet`). Devine
  muxes subs + chapters with mkvmerge; porting that to the merge path (mp4ff
  supports stpp/wvtt) is a real feature.

- [ ] **Provider capability metadata** (rosso)
  Add `catalogSize`, `maxResolution`, `hdr`, `contentTypes` to `oas.Service`.
  Rosso's provider table is the model; feeds both the UI and routing logic.
