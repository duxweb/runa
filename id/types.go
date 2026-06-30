package id

import "context"

// Generator creates unique IDs.
type Generator interface {
	New(ctx context.Context) (uint64, error)
	String(ctx context.Context) (string, error)
}

// GeneratorFunc adapts a function to Generator.
type GeneratorFunc func(context.Context) (uint64, error)

// New creates a new ID.
func (fn GeneratorFunc) New(ctx context.Context) (uint64, error) {
	return fn(ctx)
}

// String creates a new decimal string ID.
func (fn GeneratorFunc) String(ctx context.Context) (string, error) {
	value, err := fn(ctx)
	if err != nil {
		return "", err
	}
	return Format(value), nil
}
