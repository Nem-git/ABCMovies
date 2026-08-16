# DRM

ABCMovies supports DRM-protected content in two ways:

1. **Server-side decryption** (`internal/drm`): the convert pipeline resolves
   content keys from the init segment and decrypts CENC/CBCS fMP4 streams
   in-process before transmuxing to MP4.
2. **Browser playback** (license proxying, planned): Shaka Player fetches the
   PSSH from the (stripped) manifest and the license proxy forwards the license
   request upstream.

## Server-side decryption (implemented)

The `internal/drm` package has two layers:

- **Key providers** (`KeyProvider`) turn a `KeyRequest` (PSSH + KIDs) into
  content keys (`KID -> CEK`). Implementations exist for Widevine (via diana or
  gowidevine), PlayReady (via diana), ClearKey and AES-128.
- **Vault**: an in-memory cache (TTL + singleflight) wrapping a provider so
  repeat key requests never hit the license server twice.
- **Engine**: auto-detects the scheme from the init segment's PSSH system IDs,
  picks the matching provider, and uses mp4ff's `DecryptInit` /
  `DecryptSegmentWithKeys` to strip protection and decrypt in place.

The engine is wired into the convert pipeline (`internal/convert/mp4`): init
segments are run through `PrepareInit` (which strips pssh/sinf/tenc per the
"INIT Handling Modes" matrix below) and media segments through
`DecryptSegment` before concatenation or track merge. The output is a clean,
playable MP4.

### Device files

Both Widevine backends consume the same raw device files:

| File                   | Backends           |
|------------------------|--------------------|
| `device_client_id_blob`| diana, gowidevine |
| `device_private_key`   | diana, gowidevine |

PlayReady (diana) uses:

| File          | Description                     |
|---------------|---------------------------------|
| `bdevcert.dat`| PlayReady device certificate    |
| `zprivsig.dat`| Private signing key             |
| `zprivencr.dat`| Private encryption key          |

### Backend switching

`drm.widevine.backend` selects the CDM implementation:

- `diana` (default): pure-Go Widevine + PlayReady, no session state.
- `gowidevine`: full CDM session model; additionally supports `.wvd` devices
  via `FromWVD`.

Switching backends is config-only — both consume the same raw device files.

### Config

See `CONFIG.md` → "DRM Configuration". Minimal example:

```yaml
drm:
  enabled: true
  widevine:
    backend: diana
    device_dir: devices/widevine
```

## INIT Handling Modes

### NO INIT MERGING

| # | Description                                                              | MPV | WEB |
|---|--------------------------------------------------------------------------|------|------|
| 1 | Init used to decrypt, then cleaned of PSSH/decryption info              | U    | U    |
| 2 | Init used to decrypt, original retained                                 | Y    | N    |

### INIT MERGING

| # | Description                                                              | MPV | WEB |
|---|--------------------------------------------------------------------------|------|------|
| 1 | Init merged with segment for decryption, then cleaned                   | U    | U    |
| 2 | Init merged with segment for decryption, original retained              | E    | E    |

The convert pipeline implements **NO INIT MERGING #1**: the init is used to
decrypt, then cleaned of PSSH/decryption info.
