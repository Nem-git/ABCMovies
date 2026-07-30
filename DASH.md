# DASH Proxy Design

## Proxy URL Scheme

The proxy rewrites all URLs in DASH MPD manifests so that every URL resolves to the proxy, not the upstream CDN. `$RepresentationID$` and `$Bandwidth$` are always known at MPD parse time (mandatory attributes per ISO/IEC 23009-1, Section 5.3.4.1), so they are resolved at rewrite time and baked into the proxy URL as literal values. Only `$Number$` or `$Time$` remain as player-resolved variables.

### Media Segments

```
/services/{serviceTag}/movies/{movieId}/streams/dash/periods/{period}/adaptationSets/{as}/representations/{rep}/{segment}
```

| Parameter | Source | Description |
|---|---|---|
| `{period}` | Literal integer, baked in by proxy | Period index in the MPD |
| `{as}` | Literal integer, baked in by proxy | AdaptationSet index within the Period |
| `{rep}` | Literal string, baked in by proxy | Representation `@id` attribute |
| `{segment}` | `$Number$` or `$Time$`, resolved by player | Segment number or start time, depending on template mode |

`$Number$` and `$Time$` are mutually exclusive per template (ISO/IEC 23009-1, Section 5.3.9.4.4). The proxy inspects the upstream template at rewrite time and emits only the relevant one.

### Init Segments

```
/services/{serviceTag}/movies/{movieId}/streams/dash/periods/{period}/adaptationSets/{as}/representations/{rep}/init
```

Init segments never use `$Number$` or `$Time$` (ISO/IEC 23009-1, Section 5.3.9.4.2, Table 15: "Neither `$Number$` nor the `$Time$` identifier shall be included"). `$RepresentationID$` and `$Bandwidth$` are allowed in init URLs but resolved at rewrite time, so the init path has no player-resolved variables.

## DASH SegmentTemplate Placeholders

From ISO/IEC 23009-1, Section 5.6.4:

| Placeholder | Resolves To |
|---|---|
| `$$` | Literal `$` character (escape sequence) |
| `$RepresentationID$` | Value of `Representation@id` attribute |
| `$Bandwidth$` | Value of `Representation@bandwidth` attribute |
| `$Number$` | Segment number (from `startNumber` + index) |
| `$Time$` | Segment start time from `SegmentTimeline@t` |

`$Number$` and `$Time$` are mutually exclusive per the spec.

Placeholders can have format suffixes: `$Number%05d$` zero-pads to 5 digits.

## Manifest Rewriting

The proxy iterates the MPD tree and rewrites SegmentTemplate URLs per-Representation:

```go
for periodIndex, period := range mpd.Periods {
    for asIndex, as := range period.AdaptationSets {
        for _, rep := range as.Representations {
            st := effectiveSegmentTemplate(rep) // walks rep → as → period
            // Rewrite st.Media and st.Initialization with proxy URL pattern
        }
    }
}
```

Even if the SegmentTemplate is at Period or AdaptationSet level, we always know the period and adaptation set indices at rewrite time because we iterate top-down.

### Example

**Upstream manifest:**
```xml
<Period id="p0">
  <AdaptationSet mimeType="video/mp4">
    <SegmentTemplate
        media="$RepresentationID$/seg_$Number$.m4s"
        initialization="$RepresentationID$/init.mp4"/>
    <Representation id="v1" bandwidth="5000000"/>
    <Representation id="v2" bandwidth="2500000"/>
  </AdaptationSet>
</Period>
```

**Proxy rewrites to (Number mode — `$Number$` in template):**
```xml
<Period id="p0">
  <AdaptationSet mimeType="video/mp4">
    <SegmentTemplate
        media="periods/0/adaptationSets/0/representations/v1/$Number$/seg_$Number$.m4s"
        initialization="periods/0/adaptationSets/0/representations/v1/init.mp4"/>
    <Representation id="v1" bandwidth="5000000"/>
    <Representation id="v2" bandwidth="2500000"/>
  </AdaptationSet>
</Period>
```

`$RepresentationID$` is resolved to the literal `v1` at rewrite time. `$Bandwidth$` is resolved similarly (not present in this template, but would be substituted if used). Only `$Number$` (or `$Time$` in time-addressed streams) remains as a player-resolved variable. The `periods/0/adaptationSets/0/representations/v1` prefix is the RESTful path with literal period, adaptation set, and representation indices.

## State Storage

The proxy stores the **original upstream URL template as-is**. No decomposition, no base URL extraction, no rep ID tracking.

```go
type StreamMeta struct {
    UpstreamMediaTemplate string      // e.g., "https://cdn.example.com/movie/$RepresentationID$/seg_$Number$.m4s"
    UpstreamInitTemplate  string      // e.g., "https://cdn.example.com/movie/$RepresentationID$/init.mp4"
    Headers               http.Header
    Query                 url.Values
}
```

**State key:** `{contentKey}:{format}:{period}:{as}:{rep}`

## Segment Resolution

When the player requests a segment, it resolves `$Number$` or `$Time$` and fills in the proxy URL. The proxy:

1. Applies `path.Clean` to the request URL to sanitize path traversal
2. Extracts path parameters from the cleaned URL
3. Looks up state by `(period, as, rep)`
4. Gets the stored upstream URL template
5. Substitutes values into the template

**Media segment reconstruction:**
```go
// Step 1: Resolve placeholders that were known at parse time
upstream = strings.ReplaceAll(template, "$RepresentationID$", rep)
upstream = strings.ReplaceAll(upstream, "$Bandwidth$", bandwidth)

// Step 2: Resolve the segment placeholder (check which one the template uses)
if strings.Contains(template, "$Number$") {
    upstream = strings.ReplaceAll(upstream, "$Number$", segment)
} else if strings.Contains(template, "$Time$") {
    upstream = strings.ReplaceAll(upstream, "$Time$", segment)
}
```

`$RepresentationID$` and `$Bandwidth$` are always resolved from the stored state — the proxy knows these values at parse time. `$Number$` and `$Time$` are mutually exclusive per template, so the proxy checks the stored template to determine which one to fill with the `{segment}` value from the request path.

**Init segment reconstruction:**
```go
upstream = strings.ReplaceAll(initTemplate, "$RepresentationID$", rep)
upstream = strings.ReplaceAll(upstream, "$Bandwidth$", bandwidth)
```

Init segments never contain `$Number$` or `$Time$`, so no segment placeholder substitution is needed.

### Example

- Player requests: `GET /services/movies/123/streams/dash/periods/0/adaptationSets/0/representations/v1/7`
- Proxy applies `path.Clean`, extracts: period=0, as=0, rep=v1, segment=7
- State lookup for `(0, 0, v1)` returns: `https://cdn.example.com/movie/$RepresentationID$/seg_$Number$.m4s`
- Proxy sees `$Number$` in template → substitutes: `https://cdn.example.com/movie/v1/seg_7.m4s`
- Fetches from upstream, streams to player

## SegmentTemplate Inheritance

SegmentTemplate can appear at three levels. Children inherit from parents unless they provide their own:

| Level | Behavior |
|---|---|
| Period | Template applies to all AdaptationSets and Representations below |
| AdaptationSet | Template applies to all Representations below (overrides Period) |
| Representation | Overrides everything above |

The `dash-mpd` library's `Representation.GetSegmentTemplate()` walks up to AdaptationSet but NOT to Period. We implement the full walk ourselves:

```go
func effectiveSegmentTemplate(r *mpd.RepresentationType) *mpd.SegmentTemplateType {
    if st := r.GetSegmentTemplate(); st != nil {
        return st
    }
    a := r.Parent() // AdaptationSet
    if a == nil {
        return nil
    }
    if st := a.GetSegmentTemplate(); st != nil {
        return st
    }
    p := a.Parent() // Period
    if p == nil {
        return nil
    }
    return p.SegmentTemplate
}
```

## SegmentTimeline

SegmentTimeline explicitly lists every segment's timestamp and duration. Used for live streaming and variable-length segments.

```xml
<SegmentTemplate timescale="90000"
    media="$RepresentationID$/$Time$.m4s"
    initialization="$RepresentationID$/init.mp4">
  <SegmentTimeline>
    <S t="11771760" d="357357"/>
    <S d="360360" r="3"/>
    <S d="357357"/>
  </SegmentTimeline>
</SegmentTemplate>
```

The proxy doesn't parse the SegmentTimeline — it just rewrites the `media` template URL. The player does the timeline expansion and resolves `$Time$`.

## SegmentBase

SegmentBase is used when all segments live inside a single MP4 file, accessed via HTTP Range requests.

```xml
<Representation id="1" bandwidth="4190760">
  <BaseURL>car_cenc.mp4</BaseURL>
  <SegmentBase indexRange="2755-3230">
    <Initialization range="0-2754"/>
  </SegmentBase>
</Representation>
```

The proxy only rewrites `BaseURL`. The player sends `Range: bytes=X-Y` headers, and the proxy forwards them to upstream.

## Parser Choice

Use `github.com/Eyevinn/dash-mpd` for MPD parsing. It provides:
- Structured access to Period/AdaptationSet/Representation hierarchy
- `SegmentTemplateType` with `Media`, `Initialization`, `SegmentTimeline` fields
- `Representation.GetInit()` and `GetMedia()` for resolved URLs (when needed)
- `MPD.Write()` for encoding back to XML

Use the vendored `mpd.Write` / `mpd.ReadFromFile` — NOT stdlib `encoding/xml`, to preserve namespace prefixes.

Remove `manifestor` from the project entirely. Its `URISigner` callback only fires for absolute URLs and doesn't provide structured MPD access.

## ogen Spec Changes

Current segment endpoint:
```
/services/{serviceTag}/movies/{movieId}/streams/{format}/{rendition}/{segment}
```

Needs to change to support the new DASH RESTful path parameter structure:
```
/services/{serviceTag}/movies/{movieId}/streams/dash/periods/{period}/adaptationSets/{as}/representations/{rep}/{segment}
/services/{serviceTag}/movies/{movieId}/streams/dash/periods/{period}/adaptationSets/{as}/representations/{rep}/init
```

Same changes for episode endpoints. HLS endpoints use a different, simpler structure (see HLS.md).

## Path Traversal Sanitization

All request URLs are passed through `path.Clean` before parameter extraction. On rooted paths, `path.Clean` silently eats `../` segments that attempt to navigate above the root — it never escapes the proxy's URL space. For example:

```
path.Clean("/periods/0/adaptationSets/0/representations/v1/7/../../../etc/passwd")
= "/periods/0/adaptationSets/0/etc/passwd"
```

The `../` stays within the proxy's path hierarchy. The proxy should apply `path.Clean` to the reconstructed upstream URL as well before fetching.

## Stream Differentiation

The proxy uniquely identifies a stream by the tuple `(period index, adaptation set index, representation ID)`. This is necessary because:
- The same representation ID can exist in different AdaptationSets (video vs audio)
- The same representation ID can exist in different Periods (ad break vs main content)
