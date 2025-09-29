package api

import (
	"errors"
)

var (
	// Show errors
	ErrShowNotFound error = errors.New("no show found with requested ID")

	// Season errors
	ErrSeasonNumberInvalid error = errors.New("season number provided is invalid")

	// Episode errors
	ErrEpisodeNumberInvalid error = errors.New("episode number provided is invalid")
)
