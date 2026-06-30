package core

import "context"

var defaultContext = context.Background()

// DefaultContext returns Runa's fallback root context.
func DefaultContext() context.Context {
	return defaultContext
}

// NormalizeContext returns DefaultContext when ctx is nil.
func NormalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return defaultContext
	}
	return ctx
}
