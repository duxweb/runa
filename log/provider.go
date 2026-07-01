package log

import (
	"context"
	"log/slog"

	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

type provider struct {
	runaprovider.Base
	channels map[string][]Output
}

func Provider(options ...ProviderOption) runaprovider.Provider {
	item := &provider{channels: make(map[string][]Output)}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (provider *provider) Name() string { return "log" }

func (provider *provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return New(), nil })
	return nil
}

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	for name, outputs := range provider.channels {
		registry.Set(name, outputs...)
	}
	return ctx.RegisterRouteService(registry)
}

func (provider *provider) Boot(_ context.Context, ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	slog.SetDefault(registry.Get(DefaultName))
	return nil
}

type ProviderOption func(*provider)

func Register(name string, outputs ...Output) ProviderOption {
	return func(provider *provider) {
		if name == "" {
			name = DefaultName
		}
		provider.channels[name] = append([]Output(nil), outputs...)
	}
}
