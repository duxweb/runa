package rate

import (
	"context"
	"strings"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

type provider struct {
	runaprovider.Base
	drivers  map[string]Driver
	limiters map[string][]Option
	keys     map[string]KeySource
}

func Provider(options ...ProviderOption) runaprovider.Provider {
	item := &provider{drivers: make(map[string]Driver), limiters: make(map[string][]Option), keys: make(map[string]KeySource)}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (provider *provider) Name() string { return "rate" }

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
	for name, source := range provider.keys {
		registry.Key(name, source)
	}
	for name, options := range provider.limiters {
		registry.Rate(name, options...)
	}
	store, err := runaprovider.Invoke[*config.Store](ctx)
	if err != nil {
		return err
	}
	registry.Config(store)
	return ctx.RegisterRouteService(registry)
}

type ProviderOption func(*provider)

func RegisterDriver(name string, driver Driver) ProviderOption {
	return func(provider *provider) {
		if name != "" && driver != nil {
			provider.drivers[name] = driver
		}
	}
}

func RegisterLimiter(name string, options ...Option) ProviderOption {
	return func(provider *provider) {
		if name != "" {
			provider.limiters[name] = append([]Option(nil), options...)
		}
	}
}

func RegisterKey(name string, source KeySource) ProviderOption {
	return func(provider *provider) {
		if name != "" && source != nil {
			provider.keys[cleanKeyName(name)] = source
		}
	}
}

type limiterConfig struct {
	Driver    string        `toml:"driver"`
	Algorithm string        `toml:"algorithm"`
	Limit     int           `toml:"limit"`
	Window    time.Duration `toml:"window"`
	Burst     int           `toml:"burst"`
	Key       []string      `toml:"key"`
	Meta      core.Map      `toml:"meta"`
}

func configOptions(store *config.Store, name string, keys map[string]KeySource) []Option {
	var item limiterConfig
	ok, err := config.BindNamed(store, "rate", "limiters", name, &item)
	if err != nil || !ok {
		return nil
	}
	options := make([]Option, 0, 6+len(item.Meta))
	if item.Driver != "" {
		options = append(options, Use(item.Driver))
	}
	if item.Limit > 0 && item.Window > 0 {
		switch strings.ToLower(item.Algorithm) {
		case string(AlgorithmFixedWindow):
			options = append(options, FixedWindow(item.Limit, item.Window))
		case string(AlgorithmSlidingWindow):
			options = append(options, SlidingWindow(item.Limit, item.Window))
		default:
			options = append(options, TokenBucket(item.Limit, item.Window))
		}
	}
	if item.Burst > 0 {
		options = append(options, Burst(item.Burst))
	}
	if sources := keySources(keys, item.Key); len(sources) > 0 {
		options = append(options, Key(sources...))
	}
	for key, value := range item.Meta {
		options = append(options, Meta(key, value))
	}
	return options
}

func keySources(keys map[string]KeySource, names []string) []KeySource {
	sources := make([]KeySource, 0, len(names))
	for _, name := range names {
		if source := keys[cleanKeyName(name)]; source != nil {
			sources = append(sources, source)
		}
	}
	return sources
}

func cleanKeyName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
