package errs

import "errors"

var (
	ErrInvalidServiceTag = errors.New("service tag empty")
	ErrEmptyServiceTag   = errors.New("service tag invalid")
)
