package lock

import (
	"context"
	"fmt"
	"sort"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	iregistry "github.com/duxweb/runa/kernel/registry"
)

// Registry stores lock drivers and named lockers.
type Registry struct {
	base iregistry.Base[Driver, entry]
}

type entry struct {
	name    string
	options Options
	code    []LockerOption
}

// New creates a lock registry with the memory driver installed.
func New() *Registry {
	registry := &Registry{base: iregistry.NewBase[Driver, entry](DefaultName)}
	registry.RegisterDriver(DefaultDriver, MemoryDriver())
	registry.Locker(DefaultName, Use(DefaultDriver))
	registry.Locker(Local, Use(DefaultDriver))
	registry.Locker(Schedule, Use(DefaultDriver), Prefix("schedule:"))
	registry.Locker(Queue, Use(DefaultDriver), Prefix("queue:"))
	registry.Locker(Cache, Use(DefaultDriver), Prefix("cache:"))
	return registry
}

// RegisterDriver registers a lock driver.
func (registry *Registry) RegisterDriver(name string, driver Driver) {
	registry.base.RegisterDriver(name, driver)
}

// Locker registers a named locker.
func (registry *Registry) Locker(name string, options ...LockerOption) {
	if name == "" {
		return
	}
	registry.base.RegisterEntry(name, entry{name: name, options: applyLockerOptions(options...), code: append([]LockerOption(nil), options...)})
}

// Config applies file/env config to already registered lockers.
func (registry *Registry) Config(store *config.Store) {
	for name, item := range registry.base.Entries() {
		options := append(configOptions(store, name), item.code...)
		item.options = applyLockerOptions(options...)
		registry.base.RegisterEntry(name, item)
	}
}

func applyLockerOptions(options ...LockerOption) Options {
	opts := normalizeOptions(Options{Driver: DefaultDriver, Meta: make(core.Map)})
	for _, option := range options {
		if option != nil {
			option.ApplyLocker(&opts)
		}
	}
	return normalizeOptions(opts)
}

// Of returns a named locker.
func (registry *Registry) Of(name string) (Locker, error) {
	if name == "" {
		name = DefaultName
	}
	lockerEntry, ok := registry.base.Entry(name)
	if !ok {
		return nil, fmt.Errorf("locker %s is not registered", name)
	}
	store := registry.base.Driver(lockerEntry.options.Driver)
	if store == nil {
		return nil, fmt.Errorf("lock driver %s is not registered", lockerEntry.options.Driver)
	}
	return newLocker(name, store, lockerEntry.options), nil
}

// MustOf returns a named locker or panics.
func (registry *Registry) MustOf(name string) Locker {
	locker, err := registry.Of(name)
	if err != nil {
		panic(err)
	}
	return locker
}

// Driver returns a registered driver by name.
func (registry *Registry) Driver(name string) Driver {
	if name == "" {
		name = DefaultDriver
	}
	return registry.base.Driver(name)
}

// Info returns configured locker snapshots.
func (registry *Registry) Info() []Info {
	lockers := registry.base.Entries()
	infos := make([]Info, 0, len(lockers))
	for _, item := range lockers {
		infos = append(infos, Info{
			Name:          item.name,
			Driver:        item.options.Driver,
			Prefix:        item.options.Prefix,
			TTL:           item.options.TTL,
			Wait:          item.options.Wait,
			RetryInterval: item.options.RetryInterval,
			AutoRenew:     item.options.AutoRenew,
			Meta:          core.CloneMap(item.options.Meta),
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

// Close closes all registered drivers.
func (registry *Registry) Close(ctx context.Context) error {
	return registry.base.Close(ctx, "lock driver")
}

// Shutdown closes all lock drivers when managed by DI.
func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}
