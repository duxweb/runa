package redis

import (
	"time"

	"github.com/duxweb/runa/cluster"
	"github.com/duxweb/runa/config"
	runaprovider "github.com/duxweb/runa/provider"
	goredis "github.com/redis/go-redis/v9"
)

type provider struct {
	runaprovider.Base
	items []Option
}

// Provider registers a Redis cluster driver. It can use an injected client,
// explicit connection options, cluster.redis config, redis shared config, or defaults.
func Provider(items ...Option) runaprovider.Provider {
	return &provider{items: append([]Option(nil), items...)}
}

func (provider *provider) Name() string { return "cluster.redis" }

func (provider *provider) Priority() int { return -10 }

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*cluster.DriverRegistry](ctx)
	if err != nil {
		return err
	}
	opts := provider.resolve(ctx)
	client := opts.client
	ownsClient := false
	if client == nil {
		client = newClient(opts)
		ownsClient = true
	}
	driver := newDriver(client, opts, ownsClient)
	registry.Register(driver)
	return nil
}

func (provider *provider) resolve(ctx runaprovider.Context) redisOptions {
	opts := defaultOptions()
	selector := opts
	applyOptions(&selector, provider.items...)
	store, _ := runaprovider.Invoke[*config.Store](ctx)
	if store != nil {
		applyRedisConnectionConfig(&opts, readRedisConfig(store, sharedRedisPath(selector.useName)))
		path := selector.configPath
		if path == "" {
			path = defaultConfigPath
		}
		applyRedisConfig(&opts, readRedisConfig(store, path))
	}
	applyOptions(&opts, provider.items...)
	normalizeOptions(&opts)
	return opts
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

func readRedisConfig(store *config.Store, path string) redisConfig {
	var item redisConfig
	if store == nil || path == "" || !store.Has(path) {
		return item
	}
	_ = store.Bind(path, &item)
	return item
}

func applyRedisConnectionConfig(opts *redisOptions, item redisConfig) {
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
}

func applyRedisConfig(opts *redisOptions, item redisConfig) {
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

func newClient(opts redisOptions) *goredis.Client {
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
