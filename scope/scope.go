package scope

import (
	"context"
	"errors"
	"sync"

	"github.com/duxweb/runa/core"
)

// Cleanup is called when a scope closes.
type Cleanup func(context.Context) error

// Scope stores runtime-local state for one request, command, job, or connection callback.
type Scope struct {
	ctx      context.Context
	kind     Kind
	locals   core.Map
	cleanups []Cleanup
	closed   bool
	mu       sync.Mutex
}

// New creates a runtime scope.
func New(ctx context.Context, kind Kind) *Scope {
	return &Scope{
		ctx:    core.NormalizeContext(ctx),
		kind:   kind,
		locals: make(core.Map),
	}
}

// Context returns the scope context.
func (scope *Scope) Context() context.Context {
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.ctx
}

// SetContext replaces the scope context.
func (scope *Scope) SetContext(ctx context.Context) {
	scope.mu.Lock()
	scope.ctx = core.NormalizeContext(ctx)
	scope.mu.Unlock()
}

// Kind returns the scope type.
func (scope *Scope) Kind() Kind {
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.kind
}

// Set stores a local value.
func (scope *Scope) Set(key string, value any) {
	scope.mu.Lock()
	scope.locals[key] = value
	scope.mu.Unlock()
}

// Get reads a local value.
func (scope *Scope) Get(key string) any {
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.locals[key]
}

// GetAs reads a local value cast to T.
func (scope *Scope) GetAs[T any](key string, fallback ...T) T {
	scope.mu.Lock()
	value := scope.locals[key]
	scope.mu.Unlock()
	return core.Cast[T](value, fallback...)
}

// Delete removes a local value.
func (scope *Scope) Delete(key string) {
	scope.mu.Lock()
	delete(scope.locals, key)
	scope.mu.Unlock()
}

// Locals returns a shallow copy of scope locals.
func (scope *Scope) Locals() core.Map {
	scope.mu.Lock()
	defer scope.mu.Unlock()
	items := make(core.Map, len(scope.locals))
	for key, value := range scope.locals {
		items[key] = value
	}
	return items
}

// OnClose registers a cleanup callback.
func (scope *Scope) OnClose(cleanup Cleanup) {
	if cleanup == nil {
		return
	}
	scope.mu.Lock()
	if !scope.closed {
		scope.cleanups = append(scope.cleanups, cleanup)
	}
	scope.mu.Unlock()
}

// Close runs cleanup callbacks in reverse order and clears local state.
func (scope *Scope) Close() error {
	scope.mu.Lock()
	if scope.closed {
		scope.mu.Unlock()
		return nil
	}
	scope.closed = true
	ctx := scope.ctx
	cleanups := append([]Cleanup(nil), scope.cleanups...)
	scope.cleanups = nil
	scope.locals = make(core.Map)
	scope.mu.Unlock()

	var joined error
	for i := len(cleanups) - 1; i >= 0; i-- {
		if err := cleanups[i](ctx); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// Closed reports whether the scope has been closed.
func (scope *Scope) Closed() bool {
	scope.mu.Lock()
	defer scope.mu.Unlock()
	return scope.closed
}
