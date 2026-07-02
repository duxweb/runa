package cluster

import (
	"context"
	"errors"
	"sync"
)

// DriverRegistry stores named cluster drivers contributed by driver subpackages.
type DriverRegistry struct {
	items map[string]Driver
	mu    sync.RWMutex
}

// NewDriverRegistry creates a driver registry with the default memory driver.
func NewDriverRegistry() *DriverRegistry {
	registry := &DriverRegistry{items: make(map[string]Driver)}
	registry.Register(MemoryDriver())
	return registry
}

// Register stores a cluster driver by its own name.
func (registry *DriverRegistry) Register(driver Driver) {
	if registry == nil || driver == nil || driver.Name() == "" {
		return
	}
	registry.mu.Lock()
	registry.items[driver.Name()] = driver
	registry.mu.Unlock()
}

// Driver returns a cluster driver by name.
func (registry *DriverRegistry) Driver(name string) Driver {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.items[name]
}

// Shutdown closes registered drivers when the registry is managed by DI.
func (registry *DriverRegistry) Shutdown(ctx context.Context) error {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	drivers := make([]Driver, 0, len(registry.items))
	for _, driver := range registry.items {
		drivers = append(drivers, driver)
	}
	registry.mu.RUnlock()
	var joined error
	seen := make(map[Driver]struct{}, len(drivers))
	for _, driver := range drivers {
		if driver == nil {
			continue
		}
		if _, ok := seen[driver]; ok {
			continue
		}
		seen[driver] = struct{}{}
		joined = errors.Join(joined, driver.Close(ctx))
	}
	return joined
}
