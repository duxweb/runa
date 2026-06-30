package storage

import (
	"context"
	"fmt"
	"sort"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	iregistry "github.com/duxweb/runa/kernel/registry"
)

// Registry stores storage drivers and named disks.
type Registry struct {
	base iregistry.Base[Driver, entry]
}

type entry struct {
	name    string
	options DiskOptions
	code    []DiskOption
}

// New creates a storage registry with default disk names.
func New(options ...DriverOption) *Registry {
	registry := &Registry{base: iregistry.NewBase[Driver, entry](DiskLocal)}
	registry.RegisterDriver(DefaultDriver, LocalDriver(options...))
	registry.Disk(DiskLocal, Use(DefaultDriver))
	registry.Disk(DiskPublic, Use(DefaultDriver), Prefix("public"), Public())
	registry.Disk(DiskPrivate, Use(DefaultDriver), Prefix("private"), Private())
	registry.Disk(DiskCloud, Use(DefaultDriver))
	return registry
}

// RegisterDriver registers a storage driver.
func (registry *Registry) RegisterDriver(name string, driver Driver) {
	registry.base.RegisterDriver(name, driver)
}

// Disk registers or replaces a named disk.
func (registry *Registry) Disk(name string, options ...DiskOption) {
	if name == "" {
		return
	}
	registry.base.RegisterEntry(name, entry{name: name, options: applyDiskOptions(options...), code: append([]DiskOption(nil), options...)})
}

// Config applies file/env config to already registered disks.
func (registry *Registry) Config(store *config.Store) {
	for name, item := range registry.base.Entries() {
		options := append(configOptions(store, name), item.code...)
		item.options = applyDiskOptions(options...)
		registry.base.RegisterEntry(name, item)
	}
}

// Of returns a named disk.
func (registry *Registry) Of(name string) (Disk, error) {
	item, ok := registry.base.Entry(name)
	if !ok {
		if name == "" {
			name = registry.base.Fallback()
		}
		return nil, fmt.Errorf("storage disk %s is not registered", name)
	}
	driver := registry.base.Driver(item.options.Driver)
	if driver == nil {
		return nil, fmt.Errorf("storage driver %s is not registered", item.options.Driver)
	}
	return newDisk(item.name, driver, item.options), nil
}

// MustOf returns a named disk or panics.
func (registry *Registry) MustOf(name string) Disk {
	disk, err := registry.Of(name)
	if err != nil {
		panic(err)
	}
	return disk
}

// Driver returns a registered driver by name.
func (registry *Registry) Driver(name string) Driver {
	if name == "" {
		name = DefaultDriver
	}
	return registry.base.Driver(name)
}

// Info returns configured disk snapshots.
func (registry *Registry) Info() []Info {
	disks := registry.base.Entries()
	items := make([]Info, 0, len(disks))
	for _, item := range disks {
		items = append(items, Info{
			Name:    item.name,
			Driver:  item.options.Driver,
			Prefix:  item.options.Prefix,
			Public:  item.options.Public,
			Default: item.name == registry.base.Fallback(),
			Meta:    core.CloneMap(item.options.Meta),
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// Close closes all registered drivers once.
func (registry *Registry) Close(ctx context.Context) error {
	return registry.base.Close(ctx, "storage driver")
}

// Shutdown closes all storage drivers when managed by DI.
func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}
