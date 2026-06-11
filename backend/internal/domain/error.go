package domain

import "errors"

// ErrVideoNotFound reports that a video does not exist in the library.
var ErrVideoNotFound = errors.New("video not found")
