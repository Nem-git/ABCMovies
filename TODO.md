## Metadata improvements

- Check if there would be a standard for content ratings, and see if a library exists to parse them. Also check if ISO for it exists

### RCMedia

- Parse country name to country code
- Check if it would be interesting to add navigationFilters as Genres
- Check if it would be possible to link the trailer (prob not)
- Make all shows/seasons and episodes say that their language is english (if that's true for all Gem content, verify first)
- Most dates seem to be hard to understand. Find what's the meaning of startDate, airDate, availabilityDate, datePublished, etc
- Episode: What's the difference between metadata and metdata[media]?
- Check if there would be a way to differenciate show types (ex: movies vs documentary)
- Maybe add a backup for search (ex: 1 character search, search with old v1 search)
- Have a better way of determining if content is movie/series when searching
- Maybe creating the image url would give better results, as the API doesn't return every image type, but they all exist (background, logo, program, network)
- Parse the time to show it in a good way in the web ui (ex: https://github.com/sosodev/duration)
- Add Smooth support
- Make the urls use e01 instead of the mediaid. So need to add support for getting mediaid from showid, season and eid

## DASH Proxy

- Handle SegmentBase/BaseURL streams — these use a single file with HTTP Range requests instead of SegmentTemplate. No new endpoint needed: reuse the existing `{segment}` parameter (it captures the filename from BaseURL, same `base + segment` pattern as SegmentTemplate). Changes required:
  - Manifest rewriting: resolve BaseURL chain (MPD → Period → AS → Representation) using `url.ResolveReference()`, store directory in state, rewrite BaseURL to proxy path
  - `ServeSegment`: detect SegmentBase from state, forward player's `Range` header to upstream, return partial response with `Content-Range` / 206 status
  - Implement BaseURL resolution (RFC 3986) — the dash-mpd library does not do this, has `// TODO: Apply BaseURLs` in source

## HLS Proxy

- Handle sub-playlists with segments at different base URLs (multi-CDN). Current design stores a single `UpstreamBaseURL` per variant — works when all segments share a base URL (the common case). If segments point to different CDN hosts, the proxy uses the first segment's base URL for all, which is incorrect. Fix: per-segment URL storage (map of `segmentName → fullUpstreamURL`) at the cost of significantly more state entries.

