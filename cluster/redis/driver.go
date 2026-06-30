package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/duxweb/runa/cluster"

	goredis "github.com/redis/go-redis/v9"
)

const defaultRedisPrefix = "runa:cluster"

// Option configures a Redis cluster driver.
type Option interface {
	applyRedis(*redisOptions)
}

type redisOptionFunc func(*redisOptions)

func (fn redisOptionFunc) applyRedis(options *redisOptions) { fn(options) }

type redisOptions struct {
	prefix string
	now    func() time.Time
}

// Prefix sets the Redis key prefix.
func Prefix(value string) Option {
	return redisOptionFunc(func(options *redisOptions) {
		value = strings.TrimSpace(value)
		if value != "" {
			options.prefix = strings.TrimRight(value, ":")
		}
	})
}

// Clock sets the Redis driver clock.
func Clock(fn func() time.Time) Option {
	return redisOptionFunc(func(options *redisOptions) {
		if fn != nil {
			options.now = fn
		}
	})
}

// Driver creates a Redis-backed cluster driver.
func Driver(client *goredis.Client, items ...Option) cluster.Driver {
	options := redisOptions{
		prefix: defaultRedisPrefix,
		now:    time.Now,
	}
	for _, item := range items {
		if item != nil {
			item.applyRedis(&options)
		}
	}
	return &redisDriver{client: client, options: options}
}

type redisDriver struct {
	client  *goredis.Client
	options redisOptions
}

func (driver *redisDriver) Name() string { return "redis" }

func (driver *redisDriver) Register(ctx context.Context, instance cluster.Instance) error {
	if driver.client == nil {
		return fmt.Errorf("cluster redis client is nil")
	}
	now := driver.options.now()
	if instance.StartedAt.IsZero() {
		instance.StartedAt = now
	}
	instance.HeartbeatAt = now
	if instance.Status == "" {
		instance.Status = cluster.StatusRunning
	}
	body, err := json.Marshal(instance)
	if err != nil {
		return err
	}
	ttl := instance.TTL
	if ttl <= 0 {
		ttl = cluster.DefaultTTL
	}
	return driver.client.Set(ctx, driver.instanceKey(instance.ID), body, ttl).Err()
}

func (driver *redisDriver) Heartbeat(ctx context.Context, id string, status cluster.Status) error {
	if driver.client == nil {
		return fmt.Errorf("cluster redis client is nil")
	}
	key := driver.instanceKey(id)
	raw, err := driver.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	var instance cluster.Instance
	if err := json.Unmarshal(raw, &instance); err != nil {
		return err
	}
	if status != "" {
		instance.Status = status
	}
	instance.HeartbeatAt = driver.options.now()
	body, err := json.Marshal(instance)
	if err != nil {
		return err
	}
	ttl := instance.TTL
	if ttl <= 0 {
		ttl = cluster.DefaultTTL
	}
	return driver.client.Set(ctx, key, body, ttl).Err()
}

func (driver *redisDriver) Unregister(ctx context.Context, id string) error {
	if driver.client == nil {
		return fmt.Errorf("cluster redis client is nil")
	}
	return driver.client.Del(ctx, driver.instanceKey(id)).Err()
}

func (driver *redisDriver) Instances(ctx context.Context, service string) ([]cluster.Instance, error) {
	if driver.client == nil {
		return nil, fmt.Errorf("cluster redis client is nil")
	}
	keys, err := driver.client.Keys(ctx, driver.options.prefix+":instance:*").Result()
	if err != nil {
		return nil, err
	}
	items := make([]cluster.Instance, 0, len(keys))
	for _, key := range keys {
		raw, err := driver.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}
		var item cluster.Instance
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		if service != "" && item.Service != service {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (driver *redisDriver) Close(context.Context) error { return nil }

func (driver *redisDriver) instanceKey(id string) string {
	return driver.options.prefix + ":instance:" + id
}
