package redis

import (
	"context"
	"strconv"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/lock"
	goredis "github.com/redis/go-redis/v9"
)

// Driver creates a Redis-backed distributed lock driver.
func Driver(client *goredis.Client, options ...lock.DriverOption) lock.Driver {
	opts := lock.DriverOptions{Name: "redis", Prefix: lock.DefaultRedisPrefix}
	for _, option := range options {
		if option != nil {
			option.ApplyDriver(&opts)
		}
	}
	if opts.Prefix == "" {
		opts.Prefix = lock.DefaultRedisPrefix
	}
	return &redisStore{client: client, options: opts}
}

func newStore(client *goredis.Client, opts options, ownsClient bool) lock.Driver {
	return &redisStore{
		client:     client,
		options:    lock.DriverOptions{Name: opts.driverName, Prefix: opts.prefix},
		ownsClient: ownsClient,
	}
}

type redisStore struct {
	client     *goredis.Client
	options    lock.DriverOptions
	ownsClient bool
}

func (store *redisStore) Name() string {
	if store.options.Name != "" {
		return store.options.Name
	}
	return "redis"
}

func (store *redisStore) Try(ctx context.Context, key string, token string, ttl time.Duration) (lock.State, bool, error) {
	ctx = core.NormalizeContext(ctx)
	redisKey := store.key(key)
	ok, err := store.client.SetNX(ctx, redisKey, token, ttl).Result()
	if err != nil || !ok {
		return lock.State{}, ok, err
	}
	fencing, err := store.client.Incr(ctx, redisKey+":fencing").Result()
	if err != nil {
		_ = store.Release(ctx, key, token)
		return lock.State{}, false, err
	}
	return lock.State{
		Key:       key,
		Token:     token,
		Fencing:   uint64(fencing),
		ExpiresAt: core.Now().Add(ttl),
	}, true, nil
}

func (store *redisStore) Renew(ctx context.Context, key string, token string, ttl time.Duration) error {
	ctx = core.NormalizeContext(ctx)
	result, err := store.client.Eval(ctx, renewScript, []string{store.key(key)}, token, strconv.FormatInt(ttl.Milliseconds(), 10)).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return lock.ErrNotHeld
	}
	return nil
}

func (store *redisStore) Release(ctx context.Context, key string, token string) error {
	ctx = core.NormalizeContext(ctx)
	result, err := store.client.Eval(ctx, releaseScript, []string{store.key(key)}, token).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return lock.ErrNotHeld
	}
	return nil
}

func (store *redisStore) Close(context.Context) error {
	if store.client == nil || !store.ownsClient {
		return nil
	}
	return store.client.Close()
}

func (store *redisStore) key(key string) string { return store.options.Prefix + key }

const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

const renewScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`
