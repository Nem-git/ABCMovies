// Package hashid provides content-derived identifiers for proxy URLs.
package hashid

import (
	"crypto/sha256"
	"encoding/hex"
)

const hashLen int = 6

// URLHash returns a short, collision-resistant identifier derived from an
// upstream URL. Two different upstream URLs never produce the same id, while
// identical URLs always do. The hash covers the full URL string (scheme, host,
// path and query) so URLs that share a basename but differ elsewhere (e.g.
// key.php?r=52 vs key.php?r=53) are distinguished.
//
// The id is the first characters of the SHA-256 digest.
func URLHash(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:hashLen])
}
