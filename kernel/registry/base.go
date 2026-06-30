package registry

import (
	"context"
	"reflect"
	"sync"
)

// Entries stores named entries with one fallback name.
type Entries[Entry any] struct {
	mu       sync.RWMutex
	entries  map[string]Entry
	fallback string
}

// NewEntries creates a named entries registry.
func NewEntries[Entry any](fallback string) Entries[Entry] {
	return Entries[Entry]{
		entries:  make(map[string]Entry),
		fallback: fallback,
	}
}

// RegisterEntry registers or replaces a named entry.
func (entries *Entries[Entry]) Register(name string, entry Entry) {
	if name == "" {
		return
	}
	entries.mu.Lock()
	entries.entries[name] = entry
	if entries.fallback == "" {
		entries.fallback = name
	}
	entries.mu.Unlock()
}

// Entry returns a named entry, applying fallback when name is empty.
func (entries *Entries[Entry]) Entry(name string) (Entry, bool) {
	entries.mu.RLock()
	defer entries.mu.RUnlock()
	if name == "" {
		name = entries.fallback
	}
	entry, ok := entries.entries[name]
	return entry, ok
}

// All returns an entries snapshot.
func (entries *Entries[Entry]) All() map[string]Entry {
	entries.mu.RLock()
	defer entries.mu.RUnlock()
	items := make(map[string]Entry, len(entries.entries))
	for name, entry := range entries.entries {
		items[name] = entry
	}
	return items
}

// Fallback returns the default entry name.
func (entries *Entries[Entry]) Fallback() string {
	entries.mu.RLock()
	defer entries.mu.RUnlock()
	return entries.fallback
}

// SetFallback sets the default entry name.
func (entries *Entries[Entry]) SetFallback(name string) {
	entries.mu.Lock()
	entries.fallback = name
	entries.mu.Unlock()
}

// Base stores named drivers and entries for package-specific registries.
type Base[Driver NamedCloser, Entry any] struct {
	mu      sync.RWMutex
	drivers map[string]Driver
	entries Entries[Entry]
}

// NewBase creates a named registry base.
func NewBase[Driver NamedCloser, Entry any](fallback string) Base[Driver, Entry] {
	return Base[Driver, Entry]{
		drivers: make(map[string]Driver),
		entries: NewEntries[Entry](fallback),
	}
}

// RegisterDriver registers or replaces a named driver.
func (base *Base[Driver, Entry]) RegisterDriver(name string, driver Driver) {
	if name == "" || isNil(driver) {
		return
	}
	base.mu.Lock()
	base.drivers[name] = driver
	base.mu.Unlock()
}

// Driver returns a registered driver.
func (base *Base[Driver, Entry]) Driver(name string) Driver {
	base.mu.RLock()
	defer base.mu.RUnlock()
	return base.drivers[name]
}

// RegisterEntry registers or replaces a named entry.
func (base *Base[Driver, Entry]) RegisterEntry(name string, entry Entry) {
	base.entries.Register(name, entry)
}

// Entry returns a named entry, applying fallback when name is empty.
func (base *Base[Driver, Entry]) Entry(name string) (Entry, bool) {
	return base.entries.Entry(name)
}

// Entries returns an entries snapshot.
func (base *Base[Driver, Entry]) Entries() map[string]Entry {
	return base.entries.All()
}

// Drivers returns a drivers snapshot.
func (base *Base[Driver, Entry]) Drivers() map[string]Driver {
	base.mu.RLock()
	defer base.mu.RUnlock()
	items := make(map[string]Driver, len(base.drivers))
	for name, driver := range base.drivers {
		items[name] = driver
	}
	return items
}

// Fallback returns the default entry name.
func (base *Base[Driver, Entry]) Fallback() string {
	return base.entries.Fallback()
}

// SetFallback sets the default entry name.
func (base *Base[Driver, Entry]) SetFallback(name string) {
	base.entries.SetFallback(name)
}

// Close closes each distinct registered driver once.
func (base *Base[Driver, Entry]) Close(ctx context.Context, kind string) error {
	return CloseAll(ctx, base.Drivers(), kind)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
