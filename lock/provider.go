package lock

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
	lockers map[string][]LockerOption
}

func Provider(options ...ProviderOption) runaprovider.Provider {
	item := &provider{drivers: make(map[string]Driver), lockers: make(map[string][]LockerOption)}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (provider *provider) Name() string { return "lock" }

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
	for name, options := range provider.lockers {
		registry.Locker(name, options...)
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

func RegisterLocker(name string, options ...LockerOption) ProviderOption {
	return func(provider *provider) {
		if name != "" {
			provider.lockers[name] = append([]LockerOption(nil), options...)
		}
	}
}

type lockerConfig struct {
	Driver        string        `toml:"driver"`
	Prefix        string        `toml:"prefix"`
	TTL           time.Duration `toml:"ttl"`
	Wait          time.Duration `toml:"wait"`
	RetryInterval time.Duration `toml:"retry_interval"`
	AutoRenew     bool          `toml:"auto_renew"`
	Meta          core.Map      `toml:"meta"`
}

func configOptions(store *config.Store, name string) []LockerOption {
	var item lockerConfig
	ok, err := config.BindNamed(store, "lock", "lockers", name, &item)
	if err != nil || !ok {
		return nil
	}
	options := make([]LockerOption, 0, 7+len(item.Meta))
	if item.Driver != "" {
		options = append(options, Use(item.Driver))
	}
	if item.Prefix != "" {
		options = append(options, Prefix(item.Prefix))
	}
	if item.TTL > 0 {
		options = append(options, TTL(item.TTL))
	}
	if item.Wait > 0 {
		options = append(options, Wait(item.Wait))
	}
	if item.RetryInterval > 0 {
		options = append(options, RetryInterval(item.RetryInterval))
	}
	if item.AutoRenew {
		options = append(options, AutoRenew(true))
	}
	for key, value := range item.Meta {
		options = append(options, Meta(key, value))
	}
	return options
}
