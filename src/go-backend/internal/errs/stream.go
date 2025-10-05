package errs

import "errors"

var (
	// Dash

	// Manifest
	ErrEmptyDashManifestURL     = errors.New("no dash manifest URL")
	ErrEmptyDashManifestContent = errors.New("no dash manifest content")
)
