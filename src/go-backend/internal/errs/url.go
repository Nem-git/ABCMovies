package errs

import "errors"

var (
	ErrInvalidURL = errors.New("url provided is invalid")

	ErrEmptyBackendURLEnv  = errors.New("no backend url provided in env")
	ErrEmptyFrontendURLEnv = errors.New("no frontend url provided in env")
)
