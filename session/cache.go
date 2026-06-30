package session

import (
	"context"
	"time"

	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/core"
)

type cacheDriver struct {
	name string
	pool func() cache.Cache[core.Map]
}

// CacheDriver creates a cache-backed session driver.
func CacheDriver(name string, pool cache.Cache[core.Map]) Driver {
	return CacheDriverFrom(name, func() cache.Cache[core.Map] { return pool })
}

// CacheDriverFrom creates a cache-backed session driver from a lazy pool resolver.
func CacheDriverFrom(name string, pool func() cache.Cache[core.Map]) Driver {
	if name == "" {
		name = DriverCache
	}
	return &cacheDriver{name: name, pool: pool}
}

func (driver *cacheDriver) Name() string { return driver.name }

func (driver *cacheDriver) cache() cache.Cache[core.Map] {
	if driver.pool == nil {
		return nil
	}
	return driver.pool()
}

func (driver *cacheDriver) Load(ctx context.Context, id string) (core.Map, bool, error) {
	pool := driver.cache()
	if pool == nil {
		return nil, false, nil
	}
	value, ok, err := pool.Get(ctx, id)
	if err != nil || !ok {
		return nil, ok, err
	}
	return core.CloneMap(value), true, nil
}

func (driver *cacheDriver) Save(ctx context.Context, id string, data core.Map, ttl time.Duration) error {
	pool := driver.cache()
	if pool == nil {
		return nil
	}
	return pool.Set(ctx, id, core.CloneMap(data), ttl)
}

func (driver *cacheDriver) Delete(ctx context.Context, id string) error {
	pool := driver.cache()
	if pool == nil {
		return nil
	}
	return pool.Delete(ctx, id)
}

func (driver *cacheDriver) Close(context.Context) error { return nil }
