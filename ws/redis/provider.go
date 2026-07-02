package redis

import (
	"context"
	"time"

	"github.com/duxweb/runa/config"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/ws"
	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultSharedName = "default"
	defaultConfigPath = "ws.redis"
	defaultRedisAddr  = "127.0.0.1:6379"
)

type provider struct {
	runaprovider.Base
	items      []Option
	client     *goredis.Client
	ownsClient bool
}

// Provider configures websocket hubs with Redis broker and presence. It can use
// an injected client, explicit connection options, ws.redis config, redis shared
// config, or defaults.
func Provider(items ...Option) runaprovider.Provider {
	return &provider{items: append([]Option(nil), items...)}
}

func (provider *provider) Name() string { return "ws.redis" }

func (provider *provider) Priority() int { return 10 }

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*ws.Registry](ctx)
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
	provider.client = client
	provider.ownsClient = ownsClient
	selected := hubSet(opts.hubs)
	for _, hub := range registry.Hubs() {
		if hub == nil || (len(selected) > 0 && !selected[hub.Name()]) {
			continue
		}
		hub.Configure(ws.Config{
			Broker:   Broker(client, opts.channel),
			Presence: Presence(client, opts.presencePrefix),
		})
	}
	return nil
}

func (provider *provider) Shutdown(ctx context.Context, _ runaprovider.Context) error {
	if provider.client == nil || !provider.ownsClient {
		return nil
	}
	return provider.client.Close()
}

func (provider *provider) resolve(ctx runaprovider.Context) options {
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
	Addr           *string        `toml:"addr"`
	Username       *string        `toml:"username"`
	Password       *string        `toml:"password"`
	DB             *int           `toml:"db"`
	DialTimeout    *time.Duration `toml:"dial_timeout"`
	ReadTimeout    *time.Duration `toml:"read_timeout"`
	WriteTimeout   *time.Duration `toml:"write_timeout"`
	PoolSize       *int           `toml:"pool_size"`
	MinIdle        *int           `toml:"min_idle"`
	Hubs           []string       `toml:"hubs"`
	Channel        *string        `toml:"channel"`
	PresencePrefix *string        `toml:"presence_prefix"`
}

func defaultOptions() options {
	return options{useName: defaultSharedName, addr: defaultRedisAddr, channel: defaultChannel, presencePrefix: defaultPresencePrefix}
}

func applyOptions(opts *options, items ...Option) {
	for _, item := range items {
		if item != nil {
			item(opts)
		}
	}
}

func normalizeOptions(opts *options) {
	if opts.useName == "" {
		opts.useName = defaultSharedName
	}
	if opts.addr == "" {
		opts.addr = defaultRedisAddr
	}
	if opts.channel == "" {
		opts.channel = defaultChannel
	}
	if opts.presencePrefix == "" {
		opts.presencePrefix = defaultPresencePrefix
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

func applyRedisConnectionConfig(opts *options, item redisConfig) {
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
	if len(item.Hubs) > 0 {
		opts.hubs = append([]string(nil), item.Hubs...)
	}
	if item.Channel != nil {
		opts.channel = *item.Channel
	}
	if item.PresencePrefix != nil {
		opts.presencePrefix = *item.PresencePrefix
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

func hubSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	items := make(map[string]bool, len(names))
	for _, name := range names {
		if name != "" {
			items[name] = true
		}
	}
	return items
}

var _ runaprovider.Provider = (*provider)(nil)
