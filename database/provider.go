package database

import (
	"context"

	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

type provider struct {
	runaprovider.Base
	drivers map[string]Driver
}

func Provider(options ...ProviderOption) runaprovider.Provider {
	item := &provider{drivers: make(map[string]Driver)}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (provider *provider) Name() string { return "database" }

func (provider *provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return New(), nil })
	return nil
}

func (provider *provider) Register(ctx runaprovider.Context) error {
	if err := ctx.RegisterCommand(commands()...); err != nil {
		return err
	}
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	for name, driver := range provider.drivers {
		registry.RegisterDriver(name, driver)
	}
	return ctx.RegisterRouteService(registry)
}

func (provider *provider) Boot(ctx context.Context, app runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*Registry](app)
	if err != nil {
		return err
	}
	return registry.Open(ctx, app.App())
}
