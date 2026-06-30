package host

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Manager registers and controls host units.
type Manager struct {
	mu      sync.Mutex
	units   map[string]Unit
	order   []string
	started []string
}

// NewManager creates a host manager.
func NewManager() *Manager {
	return &Manager{
		units: make(map[string]Unit),
	}
}

// Register adds or replaces host units. Re-registering the same name keeps its original order.
func (manager *Manager) Register(units ...Unit) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	for _, unit := range units {
		if unit == nil {
			continue
		}
		name := unit.Name()
		if name == "" {
			return errors.New("host name is empty")
		}
		if _, exists := manager.units[name]; !exists {
			manager.order = append(manager.order, name)
		}
		manager.units[name] = unit
	}
	return nil
}

// Start starts selected host units. When names is empty, all registered units are started.
func (manager *Manager) Start(ctx context.Context, names ...string) error {
	units, err := manager.selectUnits(names...)
	if err != nil {
		return err
	}

	started := make([]string, 0, len(units))
	for _, item := range units {
		if err := item.unit.Start(ctx); err != nil {
			manager.markStarted(started)
			stopErr := manager.Stop(ctx)
			return errors.Join(fmt.Errorf("host %s start: %w", item.name, err), stopErr)
		}
		started = append(started, item.name)
	}
	manager.markStarted(started)
	return nil
}

// Stop drains and stops started host units in reverse start order.
func (manager *Manager) Stop(ctx context.Context) error {
	manager.mu.Lock()
	started := append([]string(nil), manager.started...)
	units := make(map[string]Unit, len(manager.units))
	for name, unit := range manager.units {
		units[name] = unit
	}
	manager.started = nil
	manager.mu.Unlock()

	var joined error
	for i := len(started) - 1; i >= 0; i-- {
		name := started[i]
		unit := units[name]
		if unit == nil {
			continue
		}
		if drainer, ok := unit.(Drainer); ok {
			if err := drainer.Drain(ctx); err != nil {
				joined = errors.Join(joined, fmt.Errorf("host %s drain: %w", name, err))
			}
		}
		if err := unit.Stop(ctx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("host %s stop: %w", name, err))
		}
	}
	return joined
}

// Info returns all host snapshots in registration order.
func (manager *Manager) Info() []Info {
	manager.mu.Lock()
	order := append([]string(nil), manager.order...)
	units := make(map[string]Unit, len(manager.units))
	for name, unit := range manager.units {
		units[name] = unit
	}
	manager.mu.Unlock()

	items := make([]Info, 0, len(order))
	for _, name := range order {
		unit := units[name]
		if unit == nil {
			continue
		}
		info := Info{Name: name, Status: unit.Status()}
		if checker, ok := unit.(Checker); ok {
			health := checker.Check(context.Background())
			if addr, ok := health.Details["addr"].(string); ok {
				info.Addr = addr
			}
		}
		items = append(items, info)
	}
	return items
}

// Status returns a host unit status by name.
func (manager *Manager) Status(name string) Status {
	manager.mu.Lock()
	unit := manager.units[name]
	manager.mu.Unlock()
	if unit == nil {
		return Stopped
	}
	return unit.Status()
}

func (manager *Manager) selectUnits(names ...string) ([]namedUnit, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	if len(names) == 0 {
		items := make([]namedUnit, 0, len(manager.order))
		for _, name := range manager.order {
			unit := manager.units[name]
			if unit != nil {
				items = append(items, namedUnit{name: name, unit: unit})
			}
		}
		return items, nil
	}

	items := make([]namedUnit, 0, len(names))
	for _, name := range names {
		unit := manager.units[name]
		if unit == nil {
			return nil, fmt.Errorf("host %s not found", name)
		}
		items = append(items, namedUnit{name: name, unit: unit})
	}
	return items, nil
}

func (manager *Manager) markStarted(names []string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	for _, name := range names {
		if !contains(manager.started, name) {
			manager.started = append(manager.started, name)
		}
	}
}

type namedUnit struct {
	name string
	unit Unit
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
