package storage

import "errors"

// Storage errors for append-only stores.
var (
	// ErrNotFound is returned when a requested record does not exist.
	ErrNotFound = errors.New("not found")

	// ErrDuplicateKey is returned when attempting to insert a record
	// with a key that already exists. Append-only stores do not allow updates.
	ErrDuplicateKey = errors.New("duplicate key: append-only store does not allow updates")

	// ErrInvalidInput is returned when input validation fails.
	ErrInvalidInput = errors.New("invalid input")
)
