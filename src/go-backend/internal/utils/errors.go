package utils

import "errors"

var ErrContextNotContainsKey = errors.New("context does not contain key")
var ErrContextValueCouldNotBeCasted = errors.New("context value does not match requested type")
