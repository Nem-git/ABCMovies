package errs

import "errors"

var (
	ErrRedisKeyDoesNotExist = errors.New("key does not exist")
	ErrRedisValueEmpty      = errors.New("value is empty")
)
