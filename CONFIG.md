# Configuration Guide

ABCMovies is configured via a YAML file (default: `config.yaml`). Pass a custom path with the `-config` flag:

```bash
go run ./cmd/ -config my-config.yaml
```

---

## Top-Level Structure

```yaml
server:
  # ... server settings

services:
  # ... service/provider entries
```

---

## Server Configuration

```yaml
server:
  port: 8080              # TCP port (default: 80)
  base_url: https://example.com  # Base URL for generating absolute links
  api_prefix: /api/v1       # URL prefix for the JSON API
```

| Field        | Type   | Default        | Description                                                                 |
|--------------|--------|----------------|-----------------------------------------------------------------------------|
| `port`       | int    | `80`           | TCP port to listen on                                                       |
| `base_url`   | string | `""`           | Base URL for absolute link generation. Set to your public URL.              |
| `api_prefix` | string | `/api/v1` | URL path prefix for the JSON API endpoints.                                 |

> **Absolute URLs in API responses**: Media URLs returned by the API (posters,
> backdrops, trailers, etc.) are always absolute. The scheme + host is taken from
> `server.base_url` when set; otherwise it falls back to the `Host` of the
> incoming request (honoring `X-Forwarded-Proto` / `X-Forwarded-Host` when
> deployed behind a reverse proxy).

### Environment Variable Overrides

| Variable    | Overrides        |
|-------------|------------------|
| `BASE_URL`  | `server.base_url`  |
| `API_PREFIX`| `server.api_prefix`|

When set, these environment variables take precedence over the YAML values.

---

## Service Configuration

Each service entry defines a streaming provider. The `services` key is an array:

```yaml
services:
  - tag: CRAV
    type: stub
    name: Crave
    description: Bell Media's Crave streaming service
    country: CA
    languages: [en, fr]

  - tag: CBC
    type: cbc
    name: CBC Gem
    country: CA
    languages: [en, fr]
    url: https://gem.cbc.ca
```

### Common Fields

| Field         | Type     | Required | Description                                          |
|---------------|----------|----------|------------------------------------------------------|
| `tag`         | string   | Yes      | Unique identifier for the service (e.g. `CRAV`, `DSNP`) |
| `type`        | string   | Yes      | Provider type: `stub` or `cbc`                       |
| `name`        | string   | No       | Human-readable service name                          |
| `description` | string   | No       | Longer description of the service                    |
| `url`         | string   | No       | Service website URL                                  |
| `country`     | string   | No       | ISO 3166-1 alpha-2 country code (e.g. `CA`, `US`)   |
| `languages`   | string[] | No       | ISO 639-1 language codes (e.g. `[en, fr]`)          |

### Provider Types

| Type   | Description                                                                 |
|--------|-----------------------------------------------------------------------------|
| `stub` | Returns inline data defined directly in the config file. Good for testing.  |
| `cbc`  | Live provider that fetches data from the CBC Gem API. Content fields are ignored. |

Any other value for `type` causes a fatal error at startup.

---

## Stub Provider Fields

When `type: stub`, you can define content inline. All content arrays are optional.

```yaml
- tag: CRAV
  type: stub
  name: Crave
  country: CA
  movies:
    - ...
  series:
    - ...
  seasons:
    - ...
  episodes:
    - ...
  streams:
    - ...
  subtitles:
    - ...
  search:
    - ...
```

---

## Content Types

### Movie

```yaml
- "@type": Movie
  id: memento
  name: Memento
  description: A man with short-term memory loss attempts to track down his wife's murderer.
  datePublished: 2000-09-05
  duration: PT1H53M
  countryOfOrigin: US
  languages: [en]
  contentRating: R
  genres: [Thriller, Mystery]
  trailer: https://example.com/trailer.mp4
```

| Field              | Type     | Required | Description                                      |
|--------------------|----------|----------|--------------------------------------------------|
| `@type`            | string   | Yes      | Must be `Movie`                                  |
| `id`               | string   | Yes      | Unique canonical identifier (path-safe slug)     |
| `name`             | string   | Yes      | Title                                            |
| `description`      | string   | No       | Plot summary                                     |
| `datePublished`    | date     | No       | ISO 8601 date (`YYYY-MM-DD`)                    |
| `duration`         | string   | No       | ISO 8601 duration (e.g. `PT1H53M`)              |
| `countryOfOrigin`  | string   | No       | ISO 3166-1 alpha-2 code                          |
| `languages`        | string[] | No       | ISO 639-1 language codes                         |
| `contentRating`    | string   | No       | Content rating (e.g. `R`, `PG-13`)              |
| `genres`           | string[] | No       | Genre classifications                            |
| `trailer`          | string   | No       | URL to trailer video                             |
| `poster`           | string   | Auto     | **Auto-generated** - do not set in config         |
| `backdrop`         | string   | Auto     | **Auto-generated** - do not set in config         |

### TVSeries

```yaml
- "@type": TVSeries
  id: the-sopranos
  name: The Sopranos
  description: A New Jersey-based Italian-American mobster drama.
  datePublished: 1999-01-10
  countryOfOrigin: US
  languages: [en]
  contentRating: MA
  genres: [Drama, Crime]
  numberOfSeasons: 6
  trailer: https://example.com/trailer.mp4
```

| Field              | Type     | Required | Description                                      |
|--------------------|----------|----------|--------------------------------------------------|
| `@type`            | string   | Yes      | Must be `TVSeries`                               |
| `id`               | string   | Yes      | Unique canonical identifier                      |
| `name`             | string   | Yes      | Title                                            |
| `description`      | string   | No       | Plot summary                                     |
| `datePublished`    | date     | No       | Premiere date                                    |
| `countryOfOrigin`  | string   | No       | ISO 3166-1 alpha-2 code                          |
| `languages`        | string[] | No       | ISO 639-1 language codes                         |
| `contentRating`    | string   | No       | Content rating                                   |
| `genres`           | string[] | No       | Genre classifications                            |
| `numberOfSeasons`  | int      | Yes      | Total number of seasons                          |
| `trailer`          | string   | No       | URL to trailer video                             |
| `poster`           | string   | Auto     | **Auto-generated** - do not set in config         |
| `backdrop`         | string   | Auto     | **Auto-generated** - do not set in config         |

### TVSeason

```yaml
- "@type": TVSeason
  id: the-sopranos-season-1
  name: Season 1
  seasonNumber: 1
  description: The introduction of Tony Soprano and his family.
  datePublished: 1999-01-10
  countryOfOrigin: US
  languages: [en]
  numberOfEpisodes: 13
  trailer: https://example.com/trailer.mp4
```

| Field              | Type     | Required | Description                                      |
|--------------------|----------|----------|--------------------------------------------------|
| `@type`            | string   | Yes      | Must be `TVSeason`                               |
| `id`               | string   | Yes      | Unique canonical identifier                      |
| `name`             | string   | Yes      | Season name                                      |
| `seasonNumber`     | int      | No       | Sequential season number                         |
| `description`      | string   | No       | Narrative arc description                        |
| `datePublished`    | date     | No       | Premiere date                                    |
| `countryOfOrigin`  | string   | No       | ISO 3166-1 alpha-2 code                          |
| `languages`        | string[] | No       | ISO 639-1 language codes                         |
| `numberOfEpisodes` | int      | No       | Episode count                                    |
| `trailer`          | string   | No       | URL to trailer video                             |
| `poster`           | string   | Auto     | **Auto-generated** - do not set in config         |
| `backdrop`         | string   | Auto     | **Auto-generated** - do not set in config         |

### TVEpisode

```yaml
- "@type": TVEpisode
  id: the-sopranos-pilot
  name: The Sopranos Pilot
  episodeNumber: 1
  description: Tony Soprano seeks therapy after a panic attack.
  datePublished: 1999-01-10
  duration: PT54M
  countryOfOrigin: US
  languages: [en]
  contentRating: MA
```

| Field            | Type     | Required | Description                                      |
|------------------|----------|----------|--------------------------------------------------|
| `@type`          | string   | Yes      | Must be `TVEpisode`                              |
| `id`             | string   | Yes      | Unique canonical identifier                      |
| `name`           | string   | Yes      | Episode title                                    |
| `episodeNumber`  | int      | No       | Sequential number within the season              |
| `description`    | string   | No       | Synopsis                                         |
| `datePublished`  | date     | No       | Air date                                         |
| `duration`       | string   | Yes      | ISO 8601 duration (e.g. `PT54M`, `PT1H`)        |
| `countryOfOrigin`| string   | No       | ISO 3166-1 alpha-2 code                          |
| `languages`      | string[] | No       | ISO 639-1 language codes                         |
| `contentRating`  | string   | No       | Content rating                                   |
| `poster`         | string   | No       | URL to episode thumbnail                         |

### VideoObject (Stream)

```yaml
- "@type": VideoObject
  id: manifest.mpd
  name: DASH Stream
  encodingFormat: application/dash+xml
```

| Field            | Type   | Required | Allowed Values                                                  |
|------------------|--------|----------|-----------------------------------------------------------------|
| `@type`          | string | Yes      | Must be `VideoObject`                                           |
| `id`             | string | Yes      | Convention: `manifest.mpd`, `master.m3u8`, `video.mp4`          |
| `name`           | string | Yes      | Human-readable label                                            |
| `encodingFormat` | enum   | Yes      | `application/dash+xml`, `application/vnd.apple.mpegurl`, `video/mp4` |

### Subtitle

```yaml
- id: en.vtt
  language: en
  name: English
  kind: default
  format: vtt
  isDefault: true
```

| Field       | Type   | Required | Allowed Values                       |
|-------------|--------|----------|--------------------------------------|
| `id`        | string | Yes      | Convention: `{language}.{format}`    |
| `language`  | string | Yes      | ISO 639-1 code (e.g. `en`, `fr`)    |
| `name`      | string | No       | Human-readable label                 |
| `kind`      | enum   | No       | `default`, `sdh`, `cc`, `forced`    |
| `format`    | enum   | Yes      | `vtt`, `srt`, `ttml`, `ass`        |
| `isDefault` | bool   | No       | Whether the player auto-enables it  |

### Search Entry

Search entries map search terms to content within the same service.

```yaml
search:
  - resourceType: Movie
    resourceId: memento
  - resourceType: TVSeries
    resourceId: the-sopranos
```

| Field          | Type   | Required | Description                             |
|----------------|--------|----------|-----------------------------------------|
| `resourceType` | string | Yes      | Must be `Movie` or `TVSeries`           |
| `resourceId`   | string | Yes      | Must match the `id` of a movie/series in the same service |

---

## Proxy Configuration

The optional `proxy` block on a service entry controls how media streams are proxied and rewritten.

```yaml
- tag: CRAV
  type: stub
  # ... other fields ...
  proxy:
    strategy: auto
    decorators: [cache, auth]
    cache:
      manifests:
        enabled: true
        ttl: 5m
        max_size: 104857600
      segments:
        enabled: true
        ttl: 30m
        max_size: 1073741824
    auth:
      headers:
        Authorization: Bearer token123
        X-Custom-Header: value
```

When no `proxy` block is provided, the service defaults to `strategy: passthrough` with no decorators.

### Strategy

| Value          | Description                                                      |
|----------------|------------------------------------------------------------------|
| `auto`         | Auto-detects format (HLS or DASH) from the stream and rewrites accordingly |
| `hls`          | Forces HLS manifest rewriting                                    |
| `dash`         | Forces DASH manifest rewriting                                   |
| `passthrough`  | No manifest rewriting (default when proxy block is absent)       |

### Decorators

Decorators are applied in order. Available decorators:

| Decorator | Status      | Description                                          |
|-----------|-------------|------------------------------------------------------|
| `cache`   | Stub/TODO   | Caching layer for manifests and segments             |
| `auth`    | Implemented | Injects configured HTTP headers into upstream requests |
| `drm`     | Stub/TODO   | DRM processing (reserved for future use)             |

### Cache Configuration

| Field                    | Type   | Description                        |
|--------------------------|--------|------------------------------------|
| `manifests.enabled`      | bool   | Enable manifest caching            |
| `manifests.ttl`          | string | Time-to-live (Go duration, e.g. `5m`, `1h`) |
| `manifests.max_size`     | int64  | Maximum cache size in bytes        |
| `segments.enabled`       | bool   | Enable segment caching             |
| `segments.ttl`           | string | Time-to-live (Go duration)         |
| `segments.max_size`      | int64  | Maximum cache size in bytes        |

### Auth Configuration

| Field     | Type              | Description                                                |
|-----------|-------------------|------------------------------------------------------------|
| `headers` | map[string]string | Key-value pairs of HTTP headers to inject into upstream requests |

---

## Full Example

```yaml
server:
  port: 8080
  base_url: https://movies.example.com
  api_prefix: /api/v1alpha

services:
  - tag: CRAV
    type: stub
    name: Crave
    description: Bell Media's Crave streaming service
    country: CA
    languages: [en, fr]
    movies:
      - "@type": Movie
        id: memento
        name: Memento
        languages: [en]
        genres: [Thriller, Mystery]
      - "@type": Movie
        id: inception
        name: Inception
        languages: [en, fr]
        genres: [Action, Sci-Fi]
    series:
      - "@type": TVSeries
        id: the-sopranos
        name: The Sopranos
        numberOfSeasons: 6
    seasons:
      - "@type": TVSeason
        id: the-sopranos-season-1
        name: Season 1
        seasonNumber: 1
    episodes:
      - "@type": TVEpisode
        id: the-sopranos-pilot
        name: The Sopranos Pilot
        episodeNumber: 1
        duration: PT54M
        languages: [en]
    streams:
      - "@type": VideoObject
        id: manifest.mpd
        name: DASH Stream
        encodingFormat: application/dash+xml
    subtitles:
      - id: en.vtt
        language: en
        format: vtt
    search:
      - resourceType: Movie
        resourceId: memento

  - tag: DSNP
    type: stub
    name: Disney+
    country: US
    languages: [en]
    movies:
      - "@type": Movie
        id: the-avengers
        name: The Avengers
        languages: [en]
        genres: [Action, Superhero]
      - "@type": Movie
        id: frozen
        name: Frozen
        languages: [en, fr]
        genres: [Animation, Musical]
    series:
      - "@type": TVSeries
        id: the-mandalorian
        name: The Mandalorian
        numberOfSeasons: 3
    streams:
      - "@type": VideoObject
        id: master.m3u8
        name: HLS Stream
        encodingFormat: application/vnd.apple.mpegurl
    search:
      - resourceType: Movie
        resourceId: the-avengers

  - tag: CBC
    type: cbc
    name: CBC Gem
    description: CBC Gem streaming service from the Canadian Broadcasting Corporation
    country: CA
    languages: [en, fr]
    url: https://gem.cbc.ca
```
