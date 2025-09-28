package api

import (
	"errors"
)

var (
	// Show errors
	ErrShowNotFound error = errors.New("no show found with requested ID")
)
