package errs

import "errors"

var (
	// Show errors
	ErrShowNotFound = errors.New("no show found with requested ID")

	// Season errors
	ErrSeasonNumberInvalid = errors.New("season number provided is invalid")

	// Episode errors
	ErrEpisodeNumberInvalid = errors.New("episode number provided is invalid")
)
