package stream

import (
	"io"
	"net/http"
	"net/url"
)

// Locator describes where to find upstream stream content.
// Returned by providers instead of raw bytes.
type Locator struct {
	// URL is the upstream URL to fetch from.
	// Supports http://, https://, and file:// schemes.
	// When empty, Data should be set for direct-serve.
	URL string

	// Headers to include in all upstream requests (auth tokens, cookies, etc.).
	Headers http.Header

	// EncodingFormat is the MIME type of the content.
	// One of: "application/dash+xml", "application/vnd.apple.mpegurl", "video/mp4".
	EncodingFormat string

	// Query parameters to append to all upstream requests.
	// Useful for providers that use signed URLs with tokens as query params.
	Query url.Values

	// Data is the raw content for direct-serve when URL is empty.
	// Used by providers that hold content in-memory (e.g., stub/test providers).
	Data io.ReadCloser
}
