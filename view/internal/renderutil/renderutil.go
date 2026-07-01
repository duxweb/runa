package renderutil

import (
	"context"
	"html/template"
	"path/filepath"
	"strings"
	"time"
)

// SameModTime compares source file modification times with filesystem-second tolerance.
func SameModTime(a time.Time, b time.Time) bool {
	return a.Equal(b) || a.Truncate(time.Second).Equal(b.Truncate(time.Second))
}

// CloneFuncs clones a static template function map.
func CloneFuncs(funcs map[string]any) map[string]any {
	if len(funcs) == 0 {
		return nil
	}
	output := make(map[string]any, len(funcs))
	for name, fn := range funcs {
		output[name] = fn
	}
	return output
}

// CloneContextFuncs clones a request-scoped template function map.
func CloneContextFuncs(funcs map[string]func(context.Context) any) map[string]func(context.Context) any {
	if len(funcs) == 0 {
		return nil
	}
	output := make(map[string]func(context.Context) any, len(funcs))
	for name, fn := range funcs {
		output[name] = fn
	}
	return output
}

// BuildContextFuncMap builds render-time template functions.
func BuildContextFuncMap(ctx context.Context, funcs map[string]func(context.Context) any) template.FuncMap {
	output := make(template.FuncMap, len(funcs))
	for name, build := range funcs {
		if build == nil {
			continue
		}
		output[name] = build(ctx)
	}
	return output
}

// BuildPlaceholderFuncMap builds parse-time placeholders for request-scoped template functions.
func BuildPlaceholderFuncMap(funcs map[string]func(context.Context) any) template.FuncMap {
	output := make(template.FuncMap, len(funcs))
	for name := range funcs {
		output[name] = func(...any) any { return "" }
	}
	return output
}

// NormalizeName normalizes a template name used for lookup.
func NormalizeName(name string) string {
	name = filepath.ToSlash(strings.TrimPrefix(name, "./"))
	return strings.TrimPrefix(name, "/")
}
