package redis

import (
	"sort"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/database"
	runaprovider "github.com/duxweb/runa/provider"
)

type provider struct {
	runaprovider.Base
	connections map[string][]Option
}

// Provider registers Redis database runtimes from options or [redis.<name>] config.
func Provider(items ...ProviderOption) runaprovider.Provider {
	provider := &provider{connections: make(map[string][]Option)}
	for _, item := range items {
		if item != nil {
			item(provider)
		}
	}
	return provider
}

func (provider *provider) Name() string { return "database.redis" }

func (provider *provider) Priority() int { return 10 }

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*database.Registry](ctx)
	if err != nil {
		return err
	}
	store, _ := runaprovider.Invoke[*config.Store](ctx)
	names := provider.connectionNames(store)
	for _, name := range names {
		options := append(configOptions(store, name), provider.connections[name]...)
		registry.RegisterDriver(name, Driver(options...))
	}
	return nil
}

type ProviderOption func(*provider)

// Register registers a named Redis runtime.
func Register(name string, options ...Option) ProviderOption {
	return func(provider *provider) {
		if name == "" {
			name = database.DefaultName
		}
		provider.connections[name] = append([]Option(nil), options...)
	}
}

type providerConfig struct {
	Addr         string        `toml:"addr"`
	Username     string        `toml:"username"`
	Password     string        `toml:"password"`
	DB           int           `toml:"db"`
	Logger       string        `toml:"logger"`
	DialTimeout  time.Duration `toml:"dial_timeout"`
	ReadTimeout  time.Duration `toml:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout"`
	PoolSize     int           `toml:"pool_size"`
	MinIdle      int           `toml:"min_idle"`
}

func (provider *provider) connectionNames(store *config.Store) []string {
	seen := make(map[string]struct{}, len(provider.connections)+1)
	names := make([]string, 0, len(provider.connections)+1)
	for name := range provider.connections {
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if store != nil {
		values := store.Scope("redis").Values()
		hasRootConfig := false
		keys := make([]string, 0, len(values))
		for name, value := range values {
			if _, ok := value.(map[string]any); ok {
				keys = append(keys, name)
				continue
			}
			if isRedisConfigKey(name) {
				hasRootConfig = true
			}
		}
		sort.Strings(keys)
		for _, name := range keys {
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
		if hasRootConfig {
			if _, ok := seen[database.DefaultName]; !ok {
				names = append(names, database.DefaultName)
			}
		}
	}
	return names
}

func isRedisConfigKey(name string) bool {
	switch name {
	case "addr", "username", "password", "db", "logger", "dial_timeout", "read_timeout", "write_timeout", "pool_size", "min_idle":
		return true
	default:
		return false
	}
}

func configOptions(store *config.Store, name string) []Option {
	if store == nil {
		return nil
	}
	var item providerConfig
	ok, err := config.BindNamed(store, "redis", "", name, &item)
	if err != nil || !ok {
		if name != database.DefaultName {
			return nil
		}
		if !store.Has("redis") {
			return nil
		}
		if err := store.Bind("redis", &item); err != nil {
			return nil
		}
	}
	options := make([]Option, 0, 9)
	if item.Addr != "" {
		options = append(options, Addr(item.Addr))
	}
	if item.Username != "" || item.Password != "" {
		options = append(options, Auth(item.Username, item.Password))
	}
	if item.DB != 0 {
		options = append(options, DB(item.DB))
	}
	if item.Logger != "" {
		options = append(options, Logger(item.Logger))
	}
	if item.DialTimeout > 0 {
		options = append(options, DialTimeout(item.DialTimeout))
	}
	if item.ReadTimeout > 0 {
		options = append(options, ReadTimeout(item.ReadTimeout))
	}
	if item.WriteTimeout > 0 {
		options = append(options, WriteTimeout(item.WriteTimeout))
	}
	if item.PoolSize > 0 {
		options = append(options, PoolSize(item.PoolSize))
	}
	if item.MinIdle > 0 {
		options = append(options, MinIdle(item.MinIdle))
	}
	return options
}

var _ runaprovider.Provider = (*provider)(nil)
