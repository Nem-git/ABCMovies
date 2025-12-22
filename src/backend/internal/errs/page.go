package errs

import "errors"

var (
	ErrEmptyPageID = errors.New("page ID invalid")
)
