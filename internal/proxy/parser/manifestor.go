package parser

import (
	"fmt"

	"github.com/alanzng/manifestor/manifest"
)

// ManifestorParser wraps the manifestor library for HLS and DASH rewriting.
type ManifestorParser struct{}

func (ManifestorParser) Rewrite(data []byte, fn RewriteFunc) ([]byte, error) {
	out, err := manifest.Filter(string(data),
		manifest.WithURISigner(func(absURL string) string {
			return fn(absURL)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("manifestor rewrite: %w", err)
	}
	return []byte(out), nil
}

// NoOpParser passes manifest bytes through without rewriting.
type NoOpParser struct{}

func (NoOpParser) Rewrite(data []byte, _ RewriteFunc) ([]byte, error) {
	return data, nil
}
