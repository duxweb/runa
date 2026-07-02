package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/duxweb/runa/cluster"
	"github.com/duxweb/runa/core"
	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultDriverName  = "redis"
	defaultSharedName  = "default"
	defaultConfigPath  = "cluster.redis"
	defaultRedisAddr   = "127.0.0.1:6379"
	defaultRedisPrefix = "runa:cluster"
)

// Option configures a Redis cluster driver.
type Option interface {
	applyRedis(*redisOptions)
}

type redisOptionFunc func(*redisOptions)

func (fn redisOptionFunc) applyRedis(options *redisOptions) { fn(options) }

type redisOptions struct {
	name         string
	prefix       string
	now          func() time.Time
	configPath   string
	useName      string
	client       *goredis.Client
	addr         string
	username     string
	password     string
	db           int
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	poolSize     int
	minIdle      int
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

// Client uses an existing Redis client. The driver will not close injected clients.
func Client(client *goredis.Client) Option {
	return redisOptionFunc(func(options *redisOptions) { options.client = client })
}
func Addr(value string) Option {
	return redisOptionFunc(func(options *redisOptions) { options.addr = value })
}
func Auth(username string, password string) Option {
	return redisOptionFunc(func(options *redisOptions) { options.username, options.password = username, password })
}
func Password(value string) Option {
	return redisOptionFunc(func(options *redisOptions) { options.password = value })
}
func DB(value int) Option {
	return redisOptionFunc(func(options *redisOptions) { options.db = value })
}
func DialTimeout(value time.Duration) Option {
	return redisOptionFunc(func(options *redisOptions) { options.dialTimeout = value })
}
func ReadTimeout(value time.Duration) Option {
	return redisOptionFunc(func(options *redisOptions) { options.readTimeout = value })
}
func WriteTimeout(value time.Duration) Option {
	return redisOptionFunc(func(options *redisOptions) { options.writeTimeout = value })
}
func PoolSize(value int) Option {
	return redisOptionFunc(func(options *redisOptions) { options.poolSize = value })
}
func MinIdle(value int) Option {
	return redisOptionFunc(func(options *redisOptions) { options.minIdle = value })
}
func Config(path string) Option {
	return redisOptionFunc(func(options *redisOptions) { options.configPath = path })
}
func Use(name string) Option {
	return redisOptionFunc(func(options *redisOptions) { options.useName = name })
}
func Name(value string) Option {
	return redisOptionFunc(func(options *redisOptions) { options.name = value })
}

// Driver creates a Redis-backed cluster driver.
func Driver(client *goredis.Client, items ...Option) cluster.Driver {
	options := defaultOptions()
	applyOptions(&options, items...)
	normalizeOptions(&options)
	return &redisDriver{client: client, options: options}
}

func newDriver(client *goredis.Client, options redisOptions, ownsClient bool) cluster.Driver {
	return &redisDriver{client: client, options: options, ownsClient: ownsClient}
}

type redisDriver struct {
	client     *goredis.Client
	options    redisOptions
	ownsClient bool
}

func (driver *redisDriver) Name() string { return driver.options.name }

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

func (driver *redisDriver) Close(context.Context) error {
	if driver.client == nil || !driver.ownsClient {
		return nil
	}
	return driver.client.Close()
}

func (driver *redisDriver) instanceKey(id string) string {
	return driver.options.prefix + ":instance:" + id
}

func defaultOptions() redisOptions {
	return redisOptions{name: defaultDriverName, prefix: defaultRedisPrefix, now: core.Now, useName: defaultSharedName, addr: defaultRedisAddr}
}

func applyOptions(options *redisOptions, items ...Option) {
	for _, item := range items {
		if item != nil {
			item.applyRedis(options)
		}
	}
}

func normalizeOptions(options *redisOptions) {
	if options.name == "" {
		options.name = defaultDriverName
	}
	if options.prefix == "" {
		options.prefix = defaultRedisPrefix
	}
	options.prefix = strings.TrimRight(options.prefix, ":")
	if options.now == nil {
		options.now = core.Now
	}
	if options.useName == "" {
		options.useName = defaultSharedName
	}
	if options.addr == "" {
		options.addr = defaultRedisAddr
	}
}
