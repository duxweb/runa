package console

import (
	"context"

	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/route"
)

const consoleLocalKey = "runa.console"

type registryContextKey struct{}

type runtimeContext struct {
	app      AppContext
	config   Config
	registry *Registry
}

func withRuntime(app AppContext, config Config, registry *Registry) route.Middleware {
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			ctx.Locals(consoleLocalKey, runtimeContext{app: app, config: config, registry: registry})
			ctx.SetRequest(ctx.Request().WithContext(context.WithValue(ctx.Context(), registryContextKey{}, registry)))
			return next(ctx)
		}
	}
}

func runtimeOf(ctx *route.Context) runtimeContext {
	if ctx == nil {
		return runtimeContext{}
	}
	if value, ok := ctx.Locals(consoleLocalKey).(runtimeContext); ok {
		return value
	}
	return runtimeContext{}
}

func consoleApp(ctx *route.Context) AppContext { return runtimeOf(ctx).app }
func consoleConfig(ctx *route.Context) Config  { return runtimeOf(ctx).config }
func consoleRegistry(ctx context.Context, app AppContext) *Registry {
	if ctx != nil {
		if registry, ok := ctx.Value(registryContextKey{}).(*Registry); ok && registry != nil {
			return registry
		}
	}
	if app != nil {
		if registry, err := runaprovider.Invoke[*Registry](app); err == nil && registry != nil {
			return registry
		}
	}
	return New()
}

func consoleSummaries(ctx context.Context, app AppContext) []Summary {
	return consoleRegistry(ctx, app).Summaries(ctx, app)
}
