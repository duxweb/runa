package registry

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/duxweb/runa/core"
)

// NamedCloser is a named component with a context-aware Close hook.
type NamedCloser interface {
	Name() string
	Close(context.Context) error
}

// CloseAll closes each distinct named closer once.
func CloseAll[T NamedCloser](ctx context.Context, values map[string]T, kind string) error {
	ctx = core.NormalizeContext(ctx)
	items := make([]T, 0, len(values))
	seen := make(map[any]struct{})
	for _, value := range values {
		if key, ok := closerKey(value); ok {
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
		}
		items = append(items, value)
	}
	var joined error
	for _, item := range items {
		if err := item.Close(ctx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("close %s %s: %w", kind, item.Name(), err))
		}
	}
	return joined
}

func closerKey(value any) (any, bool) {
	reflected := reflect.ValueOf(value)
	if reflected.IsValid() && reflected.Type().Comparable() {
		return value, true
	}
	return nil, false
}
