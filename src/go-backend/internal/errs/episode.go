package errs

import "errors"

var (
	ErrEmptyEpisodeNumber   = errors.New("episode number empty")
	ErrInvalidEpisodeNumber = errors.New("episode number invalid")
)
