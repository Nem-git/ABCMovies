# DRM Behavior Matrix

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

### Legend

- **U** = Unknown
- **Y** = Works perfectly
- **N** = Does not play at all
- **E** = Plays with errors or warnings

---

## 📦 MP4 Cleanup with `mp4edit`

To remove DRM-related metadata, use the following paths with `mp4edit`:

```text
moov/pssh
moov/trak/mdia/minf/stbl/stsd/sinf
```

---

## 🧩 Stream Format Cleanup

The `[tags]` like `encv` and `enca` likely indicate encrypted (DRM-protected) streams. To properly sanitize:

- Replace them with appropriate clear formats like `avc1` (video) or `mp4a` (audio).
- Ideally, extract the `original_format` from:  
  `moov/trak/mdia/minf/stbl/stsd/sinf/frma/original_format`
- Replace the DRM tag with the value from `original_format`.

Example before:

```text
[stbl] size=8+159
  [stsd] size=12+79
    entry_count = 1
    [mp4a] size=8+67
```

Replace:

```text
moov/trak/mdia/minf/stbl/stsd/encv or enca
```

With:

```text
moov/trak/mdia/minf/stbl/stsd/[original_format]
```

---

# ✅ TODOs

## Docker Container

- [ ] Create a Docker container for self-hosting

## 🐘 PHP Backend

### General

- [ ] Add `.env` to `.gitignore` once feature-complete
- [ ] Validate HTTP requests
- [ ] Improve error detection and logging
- [ ] Return proper HTTP errors (not malformed JSON)
- [ ] Log API access
- [ ] Add more try/catch blocks
- [ ] Evaluate if `require_once` is the best practice for constants
- [ ] Use Slim groups for organizing API endpoints
- [ ] Log all database access

### `Fairplay.php`

- [ ] Implement Fairplay DRM support

### `Playready.php`

- [ ] Implement Playready DRM support

### `StreamingService.php`

- [ ] Add abstract methods for custom request headers (currently using `HTTP_DEFAULT_HEADERS`)
- [ ] Make segment decryption optional

### `ObjectFactory.php`

- [ ] Reconsider if this belongs in `Models`
- [ ] Research better object creation patterns in PHP

### `Toutv.php`

- [ ] Add login support for the streaming service
- [ ] Store login keys in Redis with JWT TTL
- [ ] Handle login logic entirely in PHP (avoid Python backend dependency)

### `SegmentDecryptor\`

- [ ] Determine how to clean PSSH box to remove DRM info

### `SegmentDecryptor\PHP.php`

- [ ] Investigate FFI to replace shell calls with direct native function access

### `SlimResponseHelper.php`

- [ ] Remove pretty printing in JSON response
- [ ] Return proper MIME types (not just `video/mp4`)

### `RequestHelper.php`

- [ ] Make HTTP requests asynchronous

### `ManifestController.php`

- [ ] Research proper naming convention for controllers
- [ ] Create a base class for repository interactions (DB-agnostic)

### `RedisRepository.php`

- [ ] Add optional TTL to all Redis entries

---

## 🌐 Svelte Frontend

### General

- [ ] Migrate from Routify to SvelteKit (new branch, large refactor)

### `SearchPage.svelte`

- [ ] Evaluate if auto-focus on hover is a good UX feature

### `NavBar.svelte`

- [ ] Fix layout bug with icon and text alignment

### `ShowPage.svelte`

- [ ] Improve layout: intuitive positioning of image, title, description at top

---

## 🔖 Code Snippets & Tags

```svelte
<picture>
  <source srcset={welcome} type="image/webp" />
  <img src={welcomeFallback} alt="Welcome" />
</picture>

<svelte:head>
  <title>Home</title>
  <meta name="description" content="Svelte demo app" />
</svelte:head>
```

---

If you have more DRM research, commands, or workflows you'd like structured this way, feel free to drop them in!
