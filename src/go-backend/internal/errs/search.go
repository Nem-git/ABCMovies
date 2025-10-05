package errs

import "errors"

var (
	ErrEmptySearchQuery = errors.New("search query empty")
)
