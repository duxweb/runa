package redis

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/core"
	goredis "github.com/redis/go-redis/v9"
)

// Driver creates a Redis-backed cache driver.
func Driver(client *goredis.Client, options ...cache.DriverOption) cache.Driver {
	opts := cache.DriverOptions{Name: "redis", Prefix: cache.DefaultRedisPref, TTL: cache.DefaultTTL}
	for _, option := range options {
		if option != nil {
			option.ApplyDriver(&opts)
		}
	}
	if opts.Prefix == "" {
		opts.Prefix = cache.DefaultRedisPref
	}
	return &redisStore{client: client, options: opts}
}

func newStore(client *goredis.Client, opts options, ownsClient bool) cache.Driver {
	return &redisStore{
		client:     client,
		options:    cache.DriverOptions{Name: opts.driverName, Prefix: opts.prefix, TTL: opts.ttl},
		ownsClient: ownsClient,
	}
}

type redisStore struct {
	client     *goredis.Client
	options    cache.DriverOptions
	ownsClient bool
	hit        atomic.Uint64
	miss       atomic.Uint64
	set        atomic.Uint64
	delete     atomic.Uint64
}

func (store *redisStore) Name() string {
	if store.options.Name != "" {
		return store.options.Name
	}
	return "redis"
}

func (store *redisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	ctx = core.NormalizeContext(ctx)
	value, err := store.client.Get(ctx, store.key(key)).Bytes()
	if err == goredis.Nil {
		store.miss.Add(1)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	store.hit.Add(1)
	return value, true, nil
}

func (store *redisStore) GetMany(ctx context.Context, keys []string) (map[string][]byte, []string, error) {
	ctx = core.NormalizeContext(ctx)
	redisKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		redisKeys = append(redisKeys, store.key(key))
	}
	items, err := store.client.MGet(ctx, redisKeys...).Result()
	if err != nil {
		return nil, nil, err
	}
	values := make(map[string][]byte)
	missing := make([]string, 0)
	for index, item := range items {
		key := keys[index]
		if item == nil {
			missing = append(missing, key)
			store.miss.Add(1)
			continue
		}
		switch value := item.(type) {
		case string:
			values[key] = []byte(value)
		case []byte:
			values[key] = append([]byte(nil), value...)
		}
		store.hit.Add(1)
	}
	return values, missing, nil
}

func (store *redisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx = core.NormalizeContext(ctx)
	if ttl <= 0 {
		ttl = store.options.TTL
	}
	if err := store.client.Set(ctx, store.key(key), value, ttl).Err(); err != nil {
		return err
	}
	store.set.Add(1)
	return nil
}

func (store *redisStore) SetMany(ctx context.Context, values map[string][]byte, ttl time.Duration) error {
	ctx = core.NormalizeContext(ctx)
	if ttl <= 0 {
		ttl = store.options.TTL
	}
	pipeline := store.client.Pipeline()
	for key, value := range values {
		pipeline.Set(ctx, store.key(key), value, ttl)
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return err
	}
	store.set.Add(uint64(len(values)))
	return nil
}

func (store *redisStore) Has(ctx context.Context, key string) (bool, error) {
	ctx = core.NormalizeContext(ctx)
	count, err := store.client.Exists(ctx, store.key(key)).Result()
	return count > 0, err
}

func (store *redisStore) Delete(ctx context.Context, keys ...string) error {
	ctx = core.NormalizeContext(ctx)
	redisKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		redisKeys = append(redisKeys, store.key(key))
	}
	if len(redisKeys) == 0 {
		return nil
	}
	count, err := store.client.Del(ctx, redisKeys...).Result()
	if err != nil {
		return err
	}
	store.delete.Add(uint64(count))
	return nil
}

func (store *redisStore) Purge(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	var cursor uint64
	for {
		keys, next, err := store.client.Scan(ctx, cursor, store.options.Prefix+"*", 1000).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := store.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			store.delete.Add(uint64(len(keys)))
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func (store *redisStore) Close(context.Context) error {
	if store.client == nil || !store.ownsClient {
		return nil
	}
	return store.client.Close()
}

func (store *redisStore) Stats(context.Context) cache.Stats {
	return cache.Stats{
		Name:   store.Name(),
		Driver: store.Name(),
		Hit:    store.hit.Load(),
		Miss:   store.miss.Load(),
		Set:    store.set.Load(),
		Delete: store.delete.Load(),
		Meta:   core.CloneMap(store.options.Meta),
	}
}

func (store *redisStore) key(key string) string { return store.options.Prefix + key }
