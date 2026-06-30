package cache

import (
	"context"
	"fmt"
	"sort"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	iregistry "github.com/duxweb/runa/kernel/registry"
)

// Registry stores cache drivers and named pools.
type Registry struct {
	base iregistry.Base[Driver, poolEntry]
}

type poolEntry struct {
	name    string
	options Options
	code    []Option
}

// New creates a cache registry with the default memory driver and default pool.
func New() *Registry {
	registry := &Registry{base: iregistry.NewBase[Driver, poolEntry](DefaultName)}
	registry.RegisterDriver(DefaultDriver, MemoryDriver())
	registry.Cache(DefaultName, Use(DefaultDriver))
	registry.Cache(Route, Use(DefaultDriver), Prefix("route:"))
	registry.Cache(Config, Use(DefaultDriver), Prefix("config:"))
	registry.Cache(View, Use(DefaultDriver), Prefix("view:"))
	registry.Cache(Permission, Use(DefaultDriver), Prefix("permission:"))
	registry.Cache(Session, Use(DefaultDriver), Prefix("session:"))
	return registry
}

// RegisterDriver registers a cache driver.
func (registry *Registry) RegisterDriver(name string, driver Driver) {
	registry.base.RegisterDriver(name, driver)
}

// Cache registers a named cache pool.
func (registry *Registry) Cache(name string, options ...Option) {
	if name == "" {
		return
	}
	registry.base.RegisterEntry(name, poolEntry{name: name, options: applyPoolOptions(options...), code: append([]Option(nil), options...)})
}

// Config applies file/env config to already registered pools.
func (registry *Registry) Config(store *config.Store) {
	for name, entry := range registry.base.Entries() {
		options := append(configOptions(store, name), entry.code...)
		entry.options = applyPoolOptions(options...)
		registry.base.RegisterEntry(name, entry)
	}
}

func applyPoolOptions(options ...Option) Options {
	opts := Options{Driver: DefaultDriver, Codec: JSONCodec(), TTL: DefaultTTL, Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyCache(&opts)
		}
	}
	return normalizeOptions(opts)
}

// Of returns a typed cache pool.
func (registry *Registry) Of[T any](name string) (Cache[T], error) {
	if name == "" {
		name = DefaultName
	}
	pool, ok := registry.base.Entry(name)
	if !ok {
		var zero Cache[T]
		return zero, fmt.Errorf("cache pool %s is not registered", name)
	}
	store := registry.base.Driver(pool.options.Driver)
	if store == nil {
		var zero Cache[T]
		return zero, fmt.Errorf("cache driver %s is not registered", pool.options.Driver)
	}
	return newTyped[T](name, store, pool.options), nil
}

// MustOf returns a typed cache pool or panics.
func (registry *Registry) MustOf[T any](name string) Cache[T] {
	cache, err := registry.Of[T](name)
	if err != nil {
		panic(err)
	}
	return cache
}

// Driver returns a registered driver by name.
func (registry *Registry) Driver(name string) Driver {
	if name == "" {
		name = DefaultDriver
	}
	return registry.base.Driver(name)
}

// Info returns configured cache pool snapshots.
func (registry *Registry) Info() []Info {
	pools := registry.base.Entries()
	infos := make([]Info, 0, len(pools))
	for _, pool := range pools {
		infos = append(infos, Info{
			Name:   pool.name,
			Driver: pool.options.Driver,
			Prefix: pool.options.Prefix,
			TTL:    pool.options.TTL,
			Meta:   core.CloneMap(pool.options.Meta),
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

// Close closes all registered drivers.
func (registry *Registry) Close(ctx context.Context) error {
	return registry.base.Close(ctx, "cache driver")
}

// Shutdown closes all cache drivers when managed by DI.
func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}
