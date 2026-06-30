package session

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	iregistry "github.com/duxweb/runa/kernel/registry"
)

type Registry struct {
	mu     sync.RWMutex
	base   iregistry.Base[Driver, entry]
	cookie CookieOptions
}

type entry struct {
	name    string
	options Options
	code    []Option
}

// New creates a registry.
func New() *Registry {
	registry := &Registry{base: iregistry.NewBase[Driver, entry](DefaultName), cookie: DefaultCookieOptions()}
	registry.RegisterDriver(DriverMemory, MemoryDriver())
	registry.Session(Web, Use(DriverMemory))
	registry.Session(Admin, Use(DriverMemory), CookieName("__Host-runa_admin_session"))
	registry.Session(API, Use(DriverMemory), CookieName("__Host-runa_api_session"))
	return registry
}

func (registry *Registry) Cookie(options ...CookieOption) {
	registry.mu.Lock()
	registry.cookie = applyCookieOptions(registry.cookie, options...)
	pools := registry.base.Entries()
	for name, pool := range pools {
		pool.options.Cookie = applyCookieOptions(registry.cookie)
		registry.base.RegisterEntry(name, pool)
	}
	registry.mu.Unlock()
}

func (registry *Registry) CookieOptions() CookieOptions {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.cookie
}

func (registry *Registry) RegisterDriver(name string, driver Driver) {
	registry.base.RegisterDriver(name, driver)
}

func (registry *Registry) Session(name string, options ...Option) {
	if name == "" {
		return
	}
	registry.base.RegisterEntry(name, entry{name: name, options: registry.applySessionOptions(options...), code: append([]Option(nil), options...)})
}

// Config applies file/env config to already registered sessions.
func (registry *Registry) Config(store *config.Store) {
	for name, item := range registry.base.Entries() {
		options := append(configOptions(store, name), item.code...)
		item.options = registry.applySessionOptions(options...)
		registry.base.RegisterEntry(name, item)
	}
}

func (registry *Registry) applySessionOptions(options ...Option) Options {
	registry.mu.RLock()
	opts := applyOptions(registry.cookie, options...)
	registry.mu.RUnlock()
	return opts
}

func (registry *Registry) Load(ctx context.Context, name string, rawCookie string, setter CookieSetter) (*Session, error) {
	ctx = core.NormalizeContext(ctx)
	pool, ok := registry.base.Entry(name)
	if !ok {
		if name == "" {
			name = registry.base.Fallback()
		}
		return nil, fmt.Errorf("session %s is not registered", name)
	}
	if name == "" {
		name = pool.name
	}
	driver := registry.base.Driver(pool.options.Driver)
	if driver == nil {
		return nil, fmt.Errorf("session driver %s is not registered", pool.options.Driver)
	}

	if stateless, ok := driver.(Stateless); ok {
		data, found, err := stateless.LoadValue(ctx, rawCookie, pool.options.Cookie)
		if err != nil {
			return nil, err
		}
		return newSession(name, "", driver, pool.options, data, !found, setter), nil
	}

	id := ""
	found := false
	if rawCookie != "" {
		if value, ok := DecodeSigned(rawCookie, pool.options.Cookie); ok {
			id = value
			found = true
		}
	}
	if id == "" {
		newID, err := newID()
		if err != nil {
			return nil, err
		}
		id = newID
	}
	data, loaded, err := driver.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	return newSession(name, id, driver, pool.options, data, !found || !loaded, setter), nil
}

func (registry *Registry) Info() []Info {
	pools := registry.base.Entries()
	items := make([]Info, 0, len(pools))
	for _, item := range pools {
		items = append(items, Info{
			Name:       item.name,
			Driver:     item.options.Driver,
			CookieName: item.options.CookieName,
			TTL:        item.options.TTL,
			Shared:     item.options.Shared,
			Default:    item.name == registry.base.Fallback(),
			Meta:       core.CloneMap(item.options.Meta),
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (registry *Registry) Options(name string) (Options, bool) {
	item, ok := registry.base.Entry(name)
	return item.options, ok
}

func (registry *Registry) Close(ctx context.Context) error {
	return registry.base.Close(ctx, "session driver")
}

// Shutdown closes all session drivers when managed by DI.
func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}
