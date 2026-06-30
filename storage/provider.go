package storage

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
	disks   map[string][]DiskOption
}

// Provider creates a storage provider.
func Provider(options ...ProviderOption) runaprovider.Provider {
	item := &provider{
		drivers: make(map[string]Driver),
		disks:   make(map[string][]DiskOption),
	}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (provider *provider) Name() string { return "storage" }

func (provider *provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) {
		return New(Root(ctx.App().(interface{ DataPath(...string) string }).DataPath("storage"))), nil
	})
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
	for name, options := range provider.disks {
		registry.Disk(name, options...)
	}
	store, err := runaprovider.Invoke[*config.Store](ctx)
	if err != nil {
		return err
	}
	registry.Config(store)
	return ctx.RegisterRouteService(registry)
}

// ProviderOption configures a storage provider.
type ProviderOption func(*provider)

// RegisterDriver registers a low-level storage driver.
func RegisterDriver(name string, driver Driver) ProviderOption {
	return func(provider *provider) {
		if name == "" || driver == nil {
			return
		}
		provider.drivers[name] = driver
	}
}

// RegisterDisk registers a named storage disk.
func RegisterDisk(name string, options ...DiskOption) ProviderOption {
	return func(provider *provider) {
		if name == "" {
			return
		}
		provider.disks[name] = append([]DiskOption(nil), options...)
	}
}

type diskConfig struct {
	Driver    string   `toml:"driver"`
	Prefix    string   `toml:"prefix"`
	Public    *bool    `toml:"public"`
	URLPrefix string   `toml:"url_prefix"`
	Domain    string   `toml:"domain"`
	Meta      core.Map `toml:"meta"`
}

func configOptions(store *config.Store, name string) []DiskOption {
	var item diskConfig
	ok, err := config.BindNamed(store, "storage", "disks", name, &item)
	if err != nil || !ok {
		return nil
	}
	options := make([]DiskOption, 0, 7+len(item.Meta))
	if item.Driver != "" {
		options = append(options, Use(item.Driver))
	}
	if item.Prefix != "" {
		options = append(options, Prefix(item.Prefix))
	}
	if item.Public != nil {
		if *item.Public {
			options = append(options, Public())
		} else {
			options = append(options, Private())
		}
	}
	if item.URLPrefix != "" {
		options = append(options, URLPrefix(item.URLPrefix))
	}
	if item.Domain != "" {
		options = append(options, Domain(item.Domain))
	}
	for key, value := range item.Meta {
		options = append(options, Meta(key, value))
	}
	return options
}
