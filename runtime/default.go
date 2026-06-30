package runtime

import (
	"context"
	"sync"

	"github.com/duxweb/runa/core"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

var defaultRuntime = struct {
	sync.RWMutex
	app      *App
	injector *do.RootScope
	ctx      context.Context
}{}

// Default returns the process-wide default application.
func Default() *App {
	defaultRuntime.RLock()
	app := defaultRuntime.app
	defaultRuntime.RUnlock()
	if app != nil {
		return app
	}
	return New()
}

// SetDefault sets the process-wide default application.
func SetDefault(app *App) *App {
	defaultRuntime.Lock()
	defaultRuntime.app = app
	defaultRuntime.injector = app.container
	defaultRuntime.ctx = app.ctx
	defaultRuntime.Unlock()
	runaprovider.SetDefaultInjector(app.container)
	return app
}

// DefaultInjector returns the default application DI container.
func DefaultInjector() do.Injector {
	defaultRuntime.RLock()
	injector := defaultRuntime.injector
	defaultRuntime.RUnlock()
	if injector != nil {
		return injector
	}
	return Default().container
}

// Run executes the default application.
func Run(ctx context.Context) error {
	return Default().Run(ctx)
}

func resetDefaultInjector() *do.RootScope {
	injector := do.New()
	defaultRuntime.Lock()
	defaultRuntime.injector = injector
	defaultRuntime.Unlock()
	runaprovider.SetDefaultInjector(injector)
	return injector
}

func defaultContext() context.Context {
	defaultRuntime.RLock()
	ctx := defaultRuntime.ctx
	defaultRuntime.RUnlock()
	if ctx != nil {
		return ctx
	}
	return core.DefaultContext()
}

func setDefaultContext(ctx context.Context) {
	defaultRuntime.Lock()
	defaultRuntime.ctx = ctx
	defaultRuntime.Unlock()
}
