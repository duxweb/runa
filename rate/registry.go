package rate

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	iregistry "github.com/duxweb/runa/kernel/registry"
)

type Registry struct {
	base iregistry.Base[Driver, entry]
	keys map[string]KeySource
}

type entry struct {
	rule Rule
	code []Option
}

func New() *Registry {
	registry := &Registry{base: iregistry.NewBase[Driver, entry](DefaultName), keys: make(map[string]KeySource)}
	registry.RegisterDriver(DefaultDriver, MemoryDriver())
	registry.Rate(API, TokenBucket(60, time.Minute))
	registry.Rate(Login, FixedWindow(5, time.Minute))
	registry.Rate(Admin, TokenBucket(120, time.Minute))
	registry.Rate(SMS, FixedWindow(3, time.Minute))
	return registry
}

func (registry *Registry) RegisterDriver(name string, driver Driver) {
	registry.base.RegisterDriver(name, driver)
}

func (registry *Registry) Rate(name string, options ...Option) {
	if name == "" {
		return
	}
	rule := applyRule(name, options...)
	registry.base.RegisterEntry(name, entry{rule: rule, code: append([]Option(nil), options...)})
}

func (registry *Registry) Key(name string, source KeySource) {
	if name == "" || source == nil {
		return
	}
	registry.keys[cleanKeyName(name)] = source
}

// Config applies file/env config to already registered limiters.
func (registry *Registry) Config(store *config.Store) {
	for name, item := range registry.base.Entries() {
		options := append(configOptions(store, name, registry.keys), item.code...)
		item.rule = applyRule(name, options...)
		registry.base.RegisterEntry(name, item)
	}
}

func (registry *Registry) Of(name string) (Limiter, error) {
	item, ok := registry.base.Entry(name)
	if !ok {
		if name == "" {
			name = registry.base.Fallback()
		}
		return nil, fmt.Errorf("rate limiter %s is not registered", name)
	}
	rule := item.rule
	driver := registry.base.Driver(rule.Driver)
	if driver == nil {
		return nil, fmt.Errorf("rate driver %s is not registered", rule.Driver)
	}
	if err := validateRule(rule); err != nil {
		return nil, err
	}
	return newLimiter(rule, driver), nil
}

func (registry *Registry) MustOf(name string) Limiter {
	limiter, err := registry.Of(name)
	if err != nil {
		panic(err)
	}
	return limiter
}

func (registry *Registry) Rule(name string) (Rule, bool) {
	item, ok := registry.base.Entry(name)
	return item.rule, ok
}

func (registry *Registry) Info() []Info {
	entries := registry.base.Entries()
	items := make([]Info, 0, len(entries))
	for _, entry := range entries {
		rule := entry.rule
		items = append(items, Info{
			Name:      rule.Name,
			Driver:    rule.Driver,
			Algorithm: rule.Algorithm,
			Limit:     rule.Limit,
			Window:    rule.Window,
			Burst:     rule.Burst,
			Default:   rule.Name == registry.base.Fallback(),
			Meta:      core.CloneMap(rule.Meta),
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (registry *Registry) Close(ctx context.Context) error {
	return registry.base.Close(ctx, "rate driver")
}

// Shutdown closes all rate drivers when managed by DI.
func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}
