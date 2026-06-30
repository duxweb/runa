package cache

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/duxweb/runa/core"
)

// LayeredDriver composes an L1 local store and an L2 shared store.
func LayeredDriver(l1 Driver, l2 Driver, options ...DriverOption) Driver {
	opts := normalizeDriverOptions(DriverOptions{Name: "layered", TTL: DefaultTTL})
	for _, option := range options {
		if option != nil {
			option.ApplyDriver(&opts)
		}
	}
	return &layeredStore{l1: l1, l2: l2, options: opts}
}

type layeredStore struct {
	l1      Driver
	l2      Driver
	options DriverOptions
	hit     atomic.Uint64
	miss    atomic.Uint64
	set     atomic.Uint64
	delete  atomic.Uint64
}

func (store *layeredStore) Name() string {
	if store.options.Name != "" {
		return store.options.Name
	}
	return "layered"
}

func (store *layeredStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	ctx = core.NormalizeContext(ctx)
	if store.l1 != nil {
		value, ok, err := store.l1.Get(ctx, key)
		if err == nil && ok {
			store.hit.Add(1)
			return value, true, nil
		}
	}
	if store.l2 == nil {
		store.miss.Add(1)
		return nil, false, nil
	}
	value, ok, err := store.l2.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		store.miss.Add(1)
		return nil, false, nil
	}
	store.hit.Add(1)
	if store.l1 != nil {
		_ = store.l1.Set(ctx, key, value, l1TTL(store.options.TTL))
	}
	return value, true, nil
}

func (store *layeredStore) GetMany(ctx context.Context, keys []string) (map[string][]byte, []string, error) {
	values := make(map[string][]byte)
	missing := make([]string, 0)
	for _, key := range keys {
		value, ok, err := store.Get(ctx, key)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			values[key] = value
		} else {
			missing = append(missing, key)
		}
	}
	return values, missing, nil
}

func (store *layeredStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx = core.NormalizeContext(ctx)
	if ttl <= 0 {
		ttl = store.options.TTL
	}
	if store.l2 != nil {
		if err := store.l2.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	if store.l1 != nil {
		if err := store.l1.Set(ctx, key, value, l1TTL(ttl)); err != nil && store.l2 == nil {
			return err
		}
	}
	store.set.Add(1)
	return nil
}

func (store *layeredStore) SetMany(ctx context.Context, values map[string][]byte, ttl time.Duration) error {
	for key, value := range values {
		if err := store.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

func (store *layeredStore) Has(ctx context.Context, key string) (bool, error) {
	_, ok, err := store.Get(ctx, key)
	return ok, err
}

func (store *layeredStore) Delete(ctx context.Context, keys ...string) error {
	ctx = core.NormalizeContext(ctx)
	if store.l2 != nil {
		if err := store.l2.Delete(ctx, keys...); err != nil {
			return err
		}
	}
	if store.l1 != nil {
		if err := store.l1.Delete(ctx, keys...); err != nil && store.l2 == nil {
			return err
		}
	}
	store.delete.Add(uint64(len(keys)))
	return nil
}

func (store *layeredStore) Purge(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	var joined error
	if store.l2 != nil {
		if err := store.l2.Purge(ctx); err != nil {
			joined = fmt.Errorf("%w; purge l2: %w", joined, err)
		}
	}
	if store.l1 != nil {
		if err := store.l1.Purge(ctx); err != nil {
			joined = fmt.Errorf("%w; purge l1: %w", joined, err)
		}
	}
	return joined
}

func (store *layeredStore) Close(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	var joined error
	if store.l1 != nil {
		if err := store.l1.Close(ctx); err != nil {
			joined = fmt.Errorf("%w; close l1: %w", joined, err)
		}
	}
	if store.l2 != nil {
		if err := store.l2.Close(ctx); err != nil {
			joined = fmt.Errorf("%w; close l2: %w", joined, err)
		}
	}
	return joined
}

func (store *layeredStore) Stats(ctx context.Context) Stats {
	return Stats{
		Name:   store.Name(),
		Driver: store.Name(),
		Hit:    store.hit.Load(),
		Miss:   store.miss.Load(),
		Set:    store.set.Load(),
		Delete: store.delete.Load(),
		Meta:   core.CloneMap(store.options.Meta),
	}
}

func l1TTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return DefaultL1TTL
	}
	if ttl < DefaultL1TTL {
		return ttl
	}
	return DefaultL1TTL
}
