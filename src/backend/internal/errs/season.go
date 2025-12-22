package errs

import "errors"

var (
	ErrEmptySeasonNumber   = errors.New("season number empty")
	ErrInvalidSeasonNumber = errors.New("season number invalid")
)
