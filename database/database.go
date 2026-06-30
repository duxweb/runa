package database

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// Register registers or replaces a named driver.
func (registry *Registry) RegisterDriver(name string, driver Driver) {
	if name == "" {
		name = DefaultName
	}
	registry.mu.Lock()
	registry.items[name] = &entry{name: name, driver: driver, status: "registered"}
	registry.mu.Unlock()
}

// Open opens all registered databases.
func (registry *Registry) Open(ctx context.Context, app any) error {
	registry.mu.Lock()
	items := make([]*entry, 0, len(registry.items))
	for _, item := range registry.items {
		items = append(items, item)
	}
	registry.mu.Unlock()
	for _, item := range items {
		if item.driver == nil {
			continue
		}
		db, err := item.driver.Open(ctx, Config{Name: item.name, App: app})
		registry.mu.Lock()
		if err != nil {
			item.status = "error"
			registry.mu.Unlock()
			return err
		}
		item.db = db
		item.status = "open"
		registry.mu.Unlock()
	}
	return nil
}

// Get returns a named database runtime.
func (registry *Registry) Get(name string) Database {
	if name == "" {
		name = DefaultName
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	item := registry.items[name]
	if item == nil {
		return nil
	}
	return item.db
}

// MustGet returns a named database runtime or panics.
func (registry *Registry) MustGet(name string) Database {
	db := registry.Get(name)
	if db == nil {
		if name == "" {
			name = DefaultName
		}
		panic(fmt.Errorf("database %s is not open", name))
	}
	return db
}

// Info returns database runtime snapshots.
func (registry *Registry) Info() []Info {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	names := make([]string, 0, len(registry.items))
	for name := range registry.items {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]Info, 0, len(registry.items))
	for _, name := range names {
		item := registry.items[name]
		if item.db != nil {
			info := item.db.Info()
			info.Status = item.status
			items = append(items, info)
			continue
		}
		items = append(items, Info{Name: item.name, Status: item.status})
	}
	return items
}

// Ping pings a named database runtime.
func (registry *Registry) Ping(ctx context.Context, name string) error {
	if name == "" {
		name = DefaultName
	}
	db := registry.Get(name)
	if db == nil {
		return fmt.Errorf("database %s is not open", name)
	}
	return db.Ping(ctx)
}

// Close closes all open database runtimes.
func (registry *Registry) Close(ctx context.Context) error {
	registry.mu.RLock()
	items := make([]*entry, 0, len(registry.items))
	for _, item := range registry.items {
		items = append(items, item)
	}
	registry.mu.RUnlock()
	var joined error
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.db == nil {
			continue
		}
		db := item.db
		registry.mu.Lock()
		item.db = nil
		item.status = "closed"
		registry.mu.Unlock()
		if err := db.Close(ctx); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// Shutdown closes open database runtimes when managed by DI.
func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}
