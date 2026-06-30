package openapi

import (
	"context"

	"github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/route"
)

type providerItem struct {
	provider.Base
	registry *Registry
	docs    []Config
}

// Provider creates an OpenAPI provider.
func Provider(docs ...Config) provider.Provider {
	return providerItem{registry: New(), docs: docs}
}

// Register creates an OpenAPI document config.
func Register(name string, options ...Option) Config {
	config := Config{Name: name, Title: name, Version: "1.0.0"}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return config
}

func (providerItem) Name() string { return "openapi" }

func (item providerItem) Init(_ context.Context, ctx provider.Context) error {
	provider.ProvideValueOnce(ctx, item.registry)
	return nil
}

func (item providerItem) Register(ctx provider.Context) error {
	registry, err := provider.Invoke[*route.Registry](ctx)
	if err != nil {
		return err
	}
	for _, doc := range item.docs {
		item.registry.MountConfig(registry, doc)
	}
	return ctx.RegisterCommand(ExportCommand(item.registry, registry))
}
