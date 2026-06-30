package schedule

import (
	"context"

	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/task"
	"github.com/samber/do/v2"
)

type provider struct{ runaprovider.Base }

// Provider registers the schedule registry.
func Provider() runaprovider.Provider { return provider{} }

func (provider) Name() string { return "schedule" }

func (provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return New(), nil })
	return nil
}

func (provider) Register(ctx runaprovider.Context) error {
	return ctx.RegisterCommand(commands()...)
}

func (provider) Boot(_ context.Context, ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	tasks, err := runaprovider.Invoke[*task.Registry](ctx)
	if err != nil {
		return err
	}
	return registry.Freeze(tasks)
}
