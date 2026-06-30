package cluster

import (
	"context"
	"sort"
	"sync"

	"github.com/duxweb/runa/core"
)

// MemoryDriver creates an in-process cluster driver.
func MemoryDriver() Driver {
	return &memoryDriver{items: make(map[string]Instance)}
}

type memoryDriver struct {
	mu    sync.Mutex
	items map[string]Instance
}

func (driver *memoryDriver) Name() string { return "memory" }

func (driver *memoryDriver) Register(ctx context.Context, instance Instance) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := core.Now()
	if instance.StartedAt.IsZero() {
		instance.StartedAt = now
	}
	instance.HeartbeatAt = now
	if instance.Status == "" {
		instance.Status = StatusRunning
	}
	driver.mu.Lock()
	driver.items[instance.ID] = cloneInstance(instance)
	driver.mu.Unlock()
	return nil
}

func (driver *memoryDriver) Heartbeat(ctx context.Context, id string, status Status) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()
	item, ok := driver.items[id]
	if !ok {
		return nil
	}
	if status != "" {
		item.Status = status
	}
	item.HeartbeatAt = core.Now()
	driver.items[id] = item
	return nil
}

func (driver *memoryDriver) Unregister(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	driver.mu.Lock()
	delete(driver.items, id)
	driver.mu.Unlock()
	return nil
}

func (driver *memoryDriver) Instances(ctx context.Context, service string) ([]Instance, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := core.Now()
	driver.mu.Lock()
	defer driver.mu.Unlock()
	items := make([]Instance, 0, len(driver.items))
	for _, item := range driver.items {
		if service != "" && item.Service != service {
			continue
		}
		if item.TTL > 0 && now.Sub(item.HeartbeatAt) > item.TTL {
			continue
		}
		items = append(items, cloneInstance(item))
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (driver *memoryDriver) Close(context.Context) error { return nil }

func cloneInstance(input Instance) Instance {
	output := input
	if input.Meta != nil {
		output.Meta = core.CloneMap(input.Meta)
	}
	return output
}
