package route

import (
	"context"
	"log/slog"
	"reflect"
	"strings"
)

// LoggerProvider returns named loggers without coupling route to a log package.
type LoggerProvider interface {
	Get(name string) *slog.Logger
}

// Renderer renders templates without coupling route to a view package.
type Renderer interface {
	RenderString(ctx interface{ Context() context.Context }, domain string, name string, data any) (string, error)
}

// AssetResolver resolves public asset URLs without coupling route to an asset package.
type AssetResolver interface {
	URL(domain string, path string) string
}

// Service returns an app-scoped service injected by the route registry.
func Service[T any](ctx *Context, name ...string) T {
	var zero T
	if ctx == nil || ctx.services == nil {
		return zero
	}
	key := ""
	if len(name) > 0 {
		key = strings.TrimSpace(name[0])
	}
	if key != "" {
		if value, ok := ctx.services[key].(T); ok {
			return value
		}
	}
	if value, ok := ctx.services[typeKeyOf[T]()].(T); ok {
		return value
	}
	for _, value := range ctx.services {
		if typed, ok := value.(T); ok {
			return typed
		}
	}
	return zero
}

func typeKeyOf[T any]() string {
	return typeKeyFromType(reflect.TypeOf((*T)(nil)).Elem())
}

func typeKey(value any) string {
	if value == nil {
		return ""
	}
	return typeKeyFromType(reflect.TypeOf(value))
}

func typeKeyFromType(typ reflect.Type) string {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.PkgPath() == "" {
		return typ.String()
	}
	return typ.PkgPath() + "." + typ.Name()
}
