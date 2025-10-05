package errs

import "errors"

var (
	// Show errors
	ErrShowNotFound = errors.New("no show found with requested ID")

	// Season errors
	ErrSeasonNumberInvalid = errors.New("season number provided is invalid")
	ErrSeasonNotFound      = errors.New("no season found with requested number")

	// Episode errors
	ErrEpisodeNumberInvalid = errors.New("episode number provided is invalid")
	ErrEpisodeNotFound      = errors.New("no episode found with requested number")
)
