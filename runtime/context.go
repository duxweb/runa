package runtime

import (
	"context"

	"github.com/duxweb/runa/core"
)

// DefaultContext returns Runa's fallback root context.
func DefaultContext() context.Context {
	return defaultContext()
}

// Context returns the application runtime context.
func (app *App) Context() context.Context {
	app.mu.Lock()
	defer app.mu.Unlock()
	if app.ctx == nil {
		return core.DefaultContext()
	}
	return app.ctx
}

// SetContext sets the application runtime context.
func (app *App) SetContext(ctx context.Context) *App {
	app.mu.Lock()
	app.ctx = normalizeContext(ctx)
	ctx = app.ctx
	app.mu.Unlock()
	setDefaultContext(ctx)
	return app
}

func (app *App) enterContext(ctx context.Context) context.Context {
	ctx = normalizeContext(ctx)
	app.mu.Lock()
	app.ctx = ctx
	app.mu.Unlock()
	return ctx
}

func normalizeContext(ctx context.Context) context.Context {
	return core.NormalizeContext(ctx)
}

func optionalContext(values ...context.Context) context.Context {
	if len(values) == 0 {
		return core.DefaultContext()
	}
	return core.NormalizeContext(values[0])
}
