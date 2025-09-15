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

## General

- [ ] Switch to Go as a backend

## Docker Container

- [ ] Try to create multiple networks
- [ ] Set an error_log in phpini
- [ ] Set php.ini using $PHP_INI_DIR

## 🐘 PHP Backend

### General

- [ ] Validate HTTP requests
- [ ] Improve error detection and logging
- [ ] Return proper HTTP errors (not malformed JSON)
- [ ] Log API access
- [ ] Add more try/catch blocks
- [ ] Use Slim groups for organizing API endpoints
- [ ] Log all database access
- [ ] Subtitles/Captions class to support external subs urls
- [ ] Rely more on caching of requests in my Redis for faster responses
- [ ] Switch to Postgresql, for its flexibility and plugins, that can replace Redis and act as a normal DB
- [ ] Save the modified manifest to the Redis


### `Fairplay.php`

- [ ] Implement Fairplay DRM support

### `Playready.php`

- [ ] Implement Playready DRM support

### `ObjectFactory.php`

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

- [ ] Add a recommendations page
- [ ] Add a streaming service page that uses the recommendations available on the streaming service to show recommendations
- [ ] Implement svelte-virtual-list for lazily loading content

### `ShowPage.svelte`

- [ ] Improve layout: intuitive positioning of image, title, description at top

### `+page.svelte` (Show page)

- [ ] Make the videoplayer page fullscreen and remove the Header and Footer. Like Toutv or Netflix
- [ ] Create destroy functions for unmounting Video Player component
- [ ] Use the show recommendations endpoint to recommend a couple of shows that relate to the one you're looking at

### `+page.svelte` (Episode page)

- [ ] Use the next episode endpoint to recommend a new episode

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
