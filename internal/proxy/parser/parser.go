package parser

// RewriteFunc is called for every URL found in a manifest.
// It receives an absolute upstream URL and must return the replacement proxy URL.
type RewriteFunc func(upstreamAbsoluteURL string) string

// Parser knows how to parse a specific manifest format and rewrite its URLs.
type Parser interface {
	// Rewrite takes upstream manifest bytes and a URL rewriter function,
	// returns rewritten bytes ready to serve to the client.
	Rewrite(data []byte, fn RewriteFunc) ([]byte, error)
}
