package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	runaconfig "github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/database"
	runlog "github.com/duxweb/runa/log"
	goredis "github.com/redis/go-redis/v9"
	"github.com/samber/do/v2"
)

// ConfigReader reads config values used by the Redis driver.
type ConfigReader interface {
	GetString(string, string) string
	GetInt(string, int) int
}

type driver struct {
	options options
	items   []Option
}

// Database creates a Redis database driver.
func Driver(items ...Option) database.Driver {
	return driver{options: options{addr: "127.0.0.1:6379", logger: runlog.Redis}, items: append([]Option(nil), items...)}
}

// Open opens a Redis database runtime.
func (driver driver) Open(ctx context.Context, config database.Config) (database.Database, error) {
	opts := driver.options
	code := driver.options
	applyOptions(&code, driver.items...)
	configPath := code.configPath
	if configPath == "" {
		configPath = "database.connections." + config.Name
	}
	if cfg := configStore(config.App); cfg != nil && configPath != "" {
		opts.addr = cfg.GetString(configPath+".addr", opts.addr)
		opts.username = cfg.GetString(configPath+".username", opts.username)
		opts.password = cfg.GetString(configPath+".password", opts.password)
		opts.db = cfg.GetInt(configPath+".db", opts.db)
		opts.logger = cfg.GetString(configPath+".logger", opts.logger)
		opts.poolSize = cfg.GetInt(configPath+".pool_size", opts.poolSize)
		opts.minIdle = cfg.GetInt(configPath+".min_idle", opts.minIdle)
	}
	applyOptions(&opts, driver.items...)
	if opts.addr == "" {
		return nil, fmt.Errorf("redis database %s addr is required", config.Name)
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:         opts.addr,
		Username:     opts.username,
		Password:     opts.password,
		DB:           opts.db,
		DialTimeout:  timeout(opts.dialTimeout),
		ReadTimeout:  timeout(opts.readTimeout),
		WriteTimeout: timeout(opts.writeTimeout),
		PoolSize:     opts.poolSize,
		MinIdleConns: opts.minIdle,
	})
	db := &RedisDatabase{name: config.Name, addr: opts.addr, client: client, meta: opts.meta}
	if err := db.Ping(ctx); err != nil {
		_ = db.Close(ctx)
		return nil, err
	}
	if loggers := logRegistry(config.App); loggers != nil {
		loggers.Get(opts.logger).LogAttrs(ctx, slog.LevelDebug, "redis connected",
			slog.String("db", config.Name),
			slog.String("addr", opts.addr),
		)
	}
	return db, nil
}

func configStore(app any) *runaconfig.Store {
	if store, ok := app.(*runaconfig.Store); ok {
		return store
	}
	if withInjector, ok := app.(interface{ Injector() do.Injector }); ok {
		store, _ := do.Invoke[*runaconfig.Store](withInjector.Injector())
		return store
	}
	if value, ok := app.(interface {
		Config(...string) *runaconfig.Store
	}); ok {
		return value.Config()
	}
	return nil
}

func logRegistry(app any) *runlog.Registry {
	if registry, ok := app.(*runlog.Registry); ok {
		return registry
	}
	if withInjector, ok := app.(interface{ Injector() do.Injector }); ok {
		registry, _ := do.Invoke[*runlog.Registry](withInjector.Injector())
		return registry
	}
	if value, ok := app.(interface{ Log() *runlog.Registry }); ok {
		return value.Log()
	}
	return nil
}

func applyOptions(opts *options, items ...Option) {
	for _, item := range items {
		if item != nil {
			item(opts)
		}
	}
}

func timeout(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return 0
}

// RedisDatabase is a Runa database runtime for Redis.
type RedisDatabase struct {
	name   string
	addr   string
	client *goredis.Client
	meta   core.Map
}

// Name returns runtime name.
func (db *RedisDatabase) Name() string { return db.name }

// Kind returns runtime kind.
func (db *RedisDatabase) Kind() string { return "redis" }

// Raw returns the raw Redis client.
func (db *RedisDatabase) Raw() any { return db.client }

// Redis returns the raw Redis client.
func (db *RedisDatabase) Redis() *goredis.Client { return db.client }

// Ping checks Redis availability.
func (db *RedisDatabase) Ping(ctx context.Context) error {
	if db.client == nil {
		return fmt.Errorf("redis database %s is not open", db.name)
	}
	return db.client.Ping(ctx).Err()
}

// Close closes Redis client.
func (db *RedisDatabase) Close(context.Context) error {
	if db.client == nil {
		return nil
	}
	return db.client.Close()
}

// Info returns runtime info.
func (db *RedisDatabase) Info() database.Info {
	return database.Info{Name: db.name, Kind: db.Kind(), Dialect: "redis", Meta: db.meta}
}
