package errs

import "errors"

var (
	ErrEmptySearchQuery   = errors.New("search query empty")
	ErrInvalidSearchQuery = errors.New("search query invalid")
)
