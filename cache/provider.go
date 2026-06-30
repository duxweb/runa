package cache

import (
	"context"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

type provider struct {
	runaprovider.Base
	drivers map[string]Driver
	pools   map[string][]Option
}

// Provider creates a cache provider.
func Provider(options ...ProviderOption) runaprovider.Provider {
	item := &provider{
		drivers: make(map[string]Driver),
		pools:   make(map[string][]Option),
	}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (provider *provider) Name() string { return "cache" }

func (provider *provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return New(), nil })
	return nil
}

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	for name, driver := range provider.drivers {
		registry.RegisterDriver(name, driver)
	}
	for name, options := range provider.pools {
		registry.Cache(name, options...)
	}
	store, err := runaprovider.Invoke[*config.Store](ctx)
	if err != nil {
		return err
	}
	registry.Config(store)
	return ctx.RegisterRouteService(registry)
}

// ProviderOption configures a cache provider.
type ProviderOption func(*provider)

// RegisterDriver registers a cache driver.
func RegisterDriver(name string, driver Driver) ProviderOption {
	return func(provider *provider) {
		if name != "" && driver != nil {
			provider.drivers[name] = driver
		}
	}
}

// RegisterPool registers a named cache pool.
func RegisterPool(name string, options ...Option) ProviderOption {
	return func(provider *provider) {
		if name != "" {
			provider.pools[name] = append([]Option(nil), options...)
		}
	}
}

type poolConfig struct {
	Driver string        `toml:"driver"`
	Prefix string        `toml:"prefix"`
	TTL    time.Duration `toml:"ttl"`
	Meta   core.Map      `toml:"meta"`
}

func configOptions(store *config.Store, name string) []Option {
	var item poolConfig
	ok, err := config.BindNamed(store, "cache", "pools", name, &item)
	if err != nil || !ok {
		return nil
	}
	options := make([]Option, 0, 4+len(item.Meta))
	if item.Driver != "" {
		options = append(options, Use(item.Driver))
	}
	if item.Prefix != "" {
		options = append(options, Prefix(item.Prefix))
	}
	if item.TTL > 0 {
		options = append(options, TTL(item.TTL))
	}
	for key, value := range item.Meta {
		options = append(options, Meta(key, value))
	}
	return options
}
