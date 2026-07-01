package message

import (
	"context"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

type provider struct {
	runaprovider.Base
	drivers map[string]Driver
	brokers map[string][]BrokerOption
}

func Provider(options ...ProviderOption) runaprovider.Provider {
	item := &provider{drivers: make(map[string]Driver), brokers: make(map[string][]BrokerOption)}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (provider *provider) Name() string { return "message" }

func (provider *provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return New(), nil })
	return nil
}

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	store, err := runaprovider.Invoke[*config.Store](ctx)
	if err != nil {
		return err
	}
	for name, driver := range provider.drivers {
		registry.RegisterDriver(name, driver)
	}
	for name, options := range provider.brokers {
		registry.Broker(name, options...)
	}
	registry.Config(store)
	return nil
}

func (provider *provider) Boot(ctx context.Context, app runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*Registry](app)
	if err != nil {
		return err
	}
	return registry.Freeze(ctx)
}

type ProviderOption func(*provider)

func RegisterDriver(name string, driver Driver) ProviderOption {
	return func(provider *provider) {
		if name != "" && driver != nil {
			provider.drivers[name] = driver
		}
	}
}

func RegisterBroker(name string, options ...BrokerOption) ProviderOption {
	return func(provider *provider) {
		if name != "" {
			provider.brokers[name] = append([]BrokerOption(nil), options...)
		}
	}
}

type brokerConfig struct {
	Driver string   `toml:"driver"`
	Meta   core.Map `toml:"meta"`
}

func configOptions(store *config.Store, name string) []BrokerOption {
	var item brokerConfig
	ok, err := config.BindNamed(store, "message", "brokers", name, &item)
	if err != nil || !ok {
		return nil
	}
	options := make([]BrokerOption, 0, 1+len(item.Meta))
	if item.Driver != "" {
		options = append(options, Use(item.Driver))
	}
	for key, value := range item.Meta {
		options = append(options, Meta(key, value))
	}
	return options
}
