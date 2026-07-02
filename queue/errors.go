package queue

import "errors"

// ErrUnsupported reports a driver operation that is intentionally unavailable.
var ErrUnsupported = errors.New("queue operation is unsupported")
