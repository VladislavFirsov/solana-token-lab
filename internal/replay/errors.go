package replay

import "errors"

// ErrInvalidOrdering is returned when events are not properly ordered.
var ErrInvalidOrdering = errors.New("events are not in deterministic order")
