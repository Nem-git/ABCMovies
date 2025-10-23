package errs

import "errors"

var (
	ErrInvalidStream = errors.New("invalid stream")

	// Dash

	// Manifest
	ErrEmptyDashManifestURL     = errors.New("no dash manifest URL")
	ErrEmptyDashManifestContent = errors.New("no dash manifest content")
)
