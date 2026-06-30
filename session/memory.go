package session

import (
	"context"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
)

type memoryDriver struct {
	options DriverOptions
	items   map[string]memoryItem
	nextGC  time.Time
	mu      sync.RWMutex
}

type memoryItem struct {
	data    core.Map
	expires time.Time
}

// MemoryDriver creates an in-process session driver.
func MemoryDriver(options ...DriverOption) Driver {
	opts := applyDriverOptions(options...)
	opts.Name = DriverMemory
	return &memoryDriver{options: opts, items: make(map[string]memoryItem)}
}

func (driver *memoryDriver) Name() string { return driver.options.Name }

func (driver *memoryDriver) Load(_ context.Context, id string) (core.Map, bool, error) {
	now := core.Now()
	driver.cleanup(now)
	driver.mu.RLock()
	item, ok := driver.items[driver.key(id)]
	driver.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !item.expires.IsZero() && now.After(item.expires) {
		_ = driver.Delete(context.Background(), id)
		return nil, false, nil
	}
	return core.CloneMap(item.data), true, nil
}

func (driver *memoryDriver) Save(_ context.Context, id string, data core.Map, ttl time.Duration) error {
	now := core.Now()
	driver.cleanup(now)
	if ttl <= 0 {
		ttl = driver.options.TTL
	}
	item := memoryItem{data: core.CloneMap(data)}
	if ttl > 0 {
		item.expires = now.Add(ttl)
	}
	driver.mu.Lock()
	driver.items[driver.key(id)] = item
	driver.mu.Unlock()
	return nil
}

func (driver *memoryDriver) Delete(_ context.Context, id string) error {
	driver.mu.Lock()
	delete(driver.items, driver.key(id))
	driver.mu.Unlock()
	return nil
}

func (driver *memoryDriver) Close(context.Context) error { return nil }

func (driver *memoryDriver) key(id string) string { return driver.options.Prefix + id }

func (driver *memoryDriver) cleanup(now time.Time) {
	driver.mu.RLock()
	if !driver.nextGC.IsZero() && now.Before(driver.nextGC) {
		driver.mu.RUnlock()
		return
	}
	driver.mu.RUnlock()

	driver.mu.Lock()
	defer driver.mu.Unlock()
	if !driver.nextGC.IsZero() && now.Before(driver.nextGC) {
		return
	}
	driver.nextGC = now.Add(time.Minute)
	for key, item := range driver.items {
		if !item.expires.IsZero() && now.After(item.expires) {
			delete(driver.items, key)
		}
	}
}
