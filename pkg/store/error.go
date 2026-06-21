package store

import (
	"errors"
)

var (
	// ErrorNotFound is returned by repositories when a document does not exist.
	ErrorNotFound = errors.New("not found")
	// ErrDuplicate is returned when a unique constraint is violated.
	ErrDuplicate = errors.New("duplicate key")
)
