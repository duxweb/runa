package redis

import (
	"context"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/queue"
	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultDriverName = "redis"
	defaultSharedName = "default"
	defaultConfigPath = "queue.redis"
	defaultRedisAddr  = "127.0.0.1:6379"
)

type provider struct {
	runaprovider.Base
	items []Option
}

// Provider registers a Redis queue driver. It can use an injected client,
// explicit connection options, queue.redis config, redis shared config, or defaults.
func Provider(items ...Option) runaprovider.Provider {
	return &provider{items: append([]Option(nil), items...)}
}

func (provider *provider) Name() string { return "queue.redis" }

func (provider *provider) Priority() int { return 10 }

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*queue.Registry](ctx)
	if err != nil {
		return err
	}
	opts, err := provider.resolve(ctx)
	if err != nil {
		return err
	}
	client := opts.client
	ownsClient := false
	if client == nil {
		client = newClient(opts)
		ownsClient = true
	}
	registry.RegisterDriver(opts.driverName, &driver{client: client, options: opts, ownsClient: ownsClient})
	return nil
}

func (provider *provider) resolve(ctx runaprovider.Context) (options, error) {
	opts := defaultOptions()
	selector := opts
	applyOptions(&selector, provider.items...)
	store, _ := runaprovider.Invoke[*config.Store](ctx)
	if store != nil {
		applyRedisConfig(&opts, readRedisConfig(store, sharedRedisPath(selector.useName)))
		path := selector.configPath
		if path == "" {
			path = defaultConfigPath
		}
		applyRedisConfig(&opts, readRedisConfig(store, path))
	}
	applyOptions(&opts, provider.items...)
	normalizeOptions(&opts)
	return opts, nil
}

type redisConfig struct {
	Addr         *string        `toml:"addr"`
	Username     *string        `toml:"username"`
	Password     *string        `toml:"password"`
	DB           *int           `toml:"db"`
	DialTimeout  *time.Duration `toml:"dial_timeout"`
	ReadTimeout  *time.Duration `toml:"read_timeout"`
	WriteTimeout *time.Duration `toml:"write_timeout"`
	PoolSize     *int           `toml:"pool_size"`
	MinIdle      *int           `toml:"min_idle"`
	Prefix       *string        `toml:"prefix"`
}

func defaultOptions() options {
	return options{
		prefix:     "runa:queue",
		driverName: defaultDriverName,
		useName:    defaultSharedName,
		addr:       defaultRedisAddr,
		now:        core.Now,
	}
}

func applyOptions(opts *options, items ...Option) {
	for _, item := range items {
		if item != nil {
			item(opts)
		}
	}
}

func normalizeOptions(opts *options) {
	if opts.driverName == "" {
		opts.driverName = defaultDriverName
	}
	if opts.useName == "" {
		opts.useName = defaultSharedName
	}
	if opts.addr == "" {
		opts.addr = defaultRedisAddr
	}
	if opts.prefix == "" {
		opts.prefix = "runa:queue"
	}
	if opts.now == nil {
		opts.now = core.Now
	}
}

func readRedisConfig(store *config.Store, path string) redisConfig {
	var item redisConfig
	if store == nil || path == "" || !store.Has(path) {
		return item
	}
	_ = store.Bind(path, &item)
	return item
}

func applyRedisConfig(opts *options, item redisConfig) {
	if item.Addr != nil {
		opts.addr = *item.Addr
	}
	if item.Username != nil {
		opts.username = *item.Username
	}
	if item.Password != nil {
		opts.password = *item.Password
	}
	if item.DB != nil {
		opts.db = *item.DB
	}
	if item.DialTimeout != nil {
		opts.dialTimeout = *item.DialTimeout
	}
	if item.ReadTimeout != nil {
		opts.readTimeout = *item.ReadTimeout
	}
	if item.WriteTimeout != nil {
		opts.writeTimeout = *item.WriteTimeout
	}
	if item.PoolSize != nil {
		opts.poolSize = *item.PoolSize
	}
	if item.MinIdle != nil {
		opts.minIdle = *item.MinIdle
	}
	if item.Prefix != nil {
		opts.prefix = *item.Prefix
	}
}

func sharedRedisPath(name string) string {
	if name == "" || name == defaultSharedName {
		return "redis"
	}
	return "redis." + name
}

func newClient(opts options) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:         opts.addr,
		Username:     opts.username,
		Password:     opts.password,
		DB:           opts.db,
		DialTimeout:  opts.dialTimeout,
		ReadTimeout:  opts.readTimeout,
		WriteTimeout: opts.writeTimeout,
		PoolSize:     opts.poolSize,
		MinIdleConns: opts.minIdle,
	})
}

var _ runaprovider.Provider = (*provider)(nil)

func (provider *provider) Shutdown(context.Context, runaprovider.Context) error { return nil }
