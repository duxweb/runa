package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxweb/runa/core"
)

type typedCache[T any] struct {
	name    string
	store   Driver
	options Options
	group   flightGroup
	hit     atomic.Uint64
	miss    atomic.Uint64
	set     atomic.Uint64
	delete  atomic.Uint64
}

func newTyped[T any](name string, store Driver, options Options) Cache[T] {
	return &typedCache[T]{name: name, store: store, options: normalizeOptions(options)}
}

func (cache *typedCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var zero T
	body, ok, err := cache.store.Get(core.NormalizeContext(ctx), cache.key(key))
	if err != nil {
		return zero, false, err
	}
	if !ok {
		cache.miss.Add(1)
		return zero, false, nil
	}
	var value T
	if err := cache.options.Codec.Unmarshal(body, &value); err != nil {
		return zero, false, err
	}
	cache.hit.Add(1)
	return value, true, nil
}

func (cache *typedCache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, []string, error) {
	storeKeys := make([]string, 0, len(keys))
	original := make(map[string]string, len(keys))
	for _, key := range keys {
		storeKey := cache.key(key)
		storeKeys = append(storeKeys, storeKey)
		original[storeKey] = key
	}
	bodies, missingStoreKeys, err := cache.store.GetMany(core.NormalizeContext(ctx), storeKeys)
	if err != nil {
		return nil, nil, err
	}
	values := make(map[string]T, len(bodies))
	for storeKey, body := range bodies {
		var value T
		if err := cache.options.Codec.Unmarshal(body, &value); err != nil {
			return nil, nil, err
		}
		values[original[storeKey]] = value
		cache.hit.Add(1)
	}
	missing := make([]string, 0, len(missingStoreKeys))
	for _, storeKey := range missingStoreKeys {
		missing = append(missing, original[storeKey])
		cache.miss.Add(1)
	}
	return values, missing, nil
}

func (cache *typedCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	body, err := cache.options.Codec.Marshal(value)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = cache.options.TTL
	}
	if err := cache.store.Set(core.NormalizeContext(ctx), cache.key(key), body, ttl); err != nil {
		return err
	}
	cache.set.Add(1)
	return nil
}

func (cache *typedCache[T]) SetMany(ctx context.Context, values map[string]T, ttl time.Duration) error {
	bodies := make(map[string][]byte, len(values))
	for key, value := range values {
		body, err := cache.options.Codec.Marshal(value)
		if err != nil {
			return err
		}
		bodies[cache.key(key)] = body
	}
	if ttl <= 0 {
		ttl = cache.options.TTL
	}
	if err := cache.store.SetMany(core.NormalizeContext(ctx), bodies, ttl); err != nil {
		return err
	}
	cache.set.Add(uint64(len(values)))
	return nil
}

func (cache *typedCache[T]) Remember(ctx context.Context, key string, ttl time.Duration, loader func(context.Context) (T, error)) (T, error) {
	if value, ok, err := cache.Get(ctx, key); err != nil || ok {
		return value, err
	}
	result, err := cache.group.Do(cache.key(key), func() (any, error) {
		if value, ok, err := cache.Get(ctx, key); err != nil || ok {
			return value, err
		}
		value, err := loader(core.NormalizeContext(ctx))
		if err != nil {
			var zero T
			return zero, err
		}
		if err := cache.Set(ctx, key, value, ttl); err != nil {
			var zero T
			return zero, err
		}
		return value, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	value, ok := result.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("cache loader returned %T, expected typed value", result)
	}
	return value, nil
}

func (cache *typedCache[T]) Has(ctx context.Context, key string) (bool, error) {
	return cache.store.Has(core.NormalizeContext(ctx), cache.key(key))
}

func (cache *typedCache[T]) Delete(ctx context.Context, keys ...string) error {
	storeKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		storeKeys = append(storeKeys, cache.key(key))
	}
	if err := cache.store.Delete(core.NormalizeContext(ctx), storeKeys...); err != nil {
		return err
	}
	cache.delete.Add(uint64(len(keys)))
	return nil
}

func (cache *typedCache[T]) Purge(ctx context.Context) error {
	return cache.store.Purge(core.NormalizeContext(ctx))
}

func (cache *typedCache[T]) Stats(ctx context.Context) Stats {
	stats := cache.store.Stats(core.NormalizeContext(ctx))
	stats.Name = cache.name
	stats.Driver = cache.store.Name()
	stats.Hit += cache.hit.Load()
	stats.Miss += cache.miss.Load()
	stats.Set += cache.set.Load()
	stats.Delete += cache.delete.Load()
	stats.Meta = core.CloneMap(cache.options.Meta)
	return stats
}

func (cache *typedCache[T]) key(key string) string {
	return cache.options.Prefix + key
}

type flightGroup struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

type flightCall struct {
	done chan struct{}
	val  any
	err  error
}

func (group *flightGroup) Do(key string, fn func() (any, error)) (value any, err error) {
	group.mu.Lock()
	if group.calls == nil {
		group.calls = make(map[string]*flightCall)
	}
	if call := group.calls[key]; call != nil {
		group.mu.Unlock()
		<-call.done
		return call.val, call.err
	}
	call := &flightCall{done: make(chan struct{})}
	group.calls[key] = call
	group.mu.Unlock()

	defer func() {
		if recovered := recover(); recovered != nil {
			call.err = fmt.Errorf("cache loader panic: %v", recovered)
		}
		group.mu.Lock()
		delete(group.calls, key)
		group.mu.Unlock()
		close(call.done)
		value, err = call.val, call.err
	}()

	call.val, call.err = fn()
	return call.val, call.err
}
