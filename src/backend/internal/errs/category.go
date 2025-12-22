package errs

import "errors"

var (
	ErrEmptyCategoryID = errors.New("category ID invalid")
)
