package oro

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	orodb "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/mysql"
	"github.com/duxweb/oro/driver/pgsql"
	"github.com/duxweb/oro/driver/sqlite"
	runaconfig "github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/database"
	runlog "github.com/duxweb/runa/log"
	"github.com/samber/do/v2"
)

// ConfigReader reads config values used by the Oro driver.
type ConfigReader interface {
	GetString(string, string) string
	GetInt(string, int) int
}

type driver struct {
	options options
	items   []Option
}

// Database creates an Oro database driver.
func Driver(items ...Option) database.Driver {
	return driver{options: options{dialect: "sqlite", logger: runlog.ORM}, items: append([]Option(nil), items...)}
}

// Open opens an Oro database runtime.
func (driver driver) Open(ctx context.Context, config database.Config) (database.Database, error) {
	opts := driver.options
	code := driver.options
	applyOptions(&code, driver.items...)
	configPath := code.configPath
	if configPath == "" {
		configPath = "database.connections." + config.Name
	}
	if cfg := configStore(config.App); cfg != nil && configPath != "" {
		opts.dialect = cfg.GetString(configPath+".dialect", opts.dialect)
		opts.dsn = cfg.GetString(configPath+".dsn", opts.dsn)
		opts.logger = cfg.GetString(configPath+".logger", opts.logger)
		opts.maxOpen = cfg.GetInt(configPath+".max_open", opts.maxOpen)
		opts.maxIdle = cfg.GetInt(configPath+".max_idle", opts.maxIdle)
	}
	applyOptions(&opts, driver.items...)
	if opts.db != nil {
		return &OroDatabase{name: config.Name, dialect: opts.dialect, db: opts.db, meta: opts.meta}, nil
	}
	if opts.dsn == "" {
		return nil, fmt.Errorf("oro database %s dsn is required", config.Name)
	}
	oroConfig := orodb.Config{
		Default:  "default",
		Location: core.Location(),
		Connections: map[string]orodb.ConnectionConfig{
			"default": {Driver: oroDriver(opts.dialect, opts.dsn)},
		},
		Pool: orodb.PoolConfig{
			MaxOpenConns:    opts.maxOpen,
			MaxIdleConns:    opts.maxIdle,
			ConnMaxLifetime: opts.maxLifetime,
		},
		LogLevel: orodb.LogLevelWarn,
		LogArgs:  opts.debug,
		Logger:   logger(config.App, opts.logger, config.Name, opts.dialect),
	}
	if opts.debug {
		oroConfig.LogLevel = orodb.LogLevelDebug
	}
	if opts.location != nil {
		oroConfig.Location = opts.location
	}
	db, err := orodb.Open(oroConfig)
	if err != nil {
		return nil, err
	}
	return &OroDatabase{name: config.Name, dialect: opts.dialect, db: db, ownsDB: true, meta: opts.meta}, nil
}

func oroDriver(dialect string, dsn string) orodb.Driver {
	switch strings.ToLower(dialect) {
	case "mysql":
		return mysql.Open(dsn)
	case "postgres", "pgsql", "postgresql":
		return pgsql.Open(dsn)
	default:
		return sqlite.Open(dsn)
	}
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

func applyOptions(opts *options, items ...Option) {
	for _, item := range items {
		if item != nil {
			item(opts)
		}
	}
}

func logger(app any, channel string, name string, dialect string) orodb.Logger {
	recorder, _ := app.(interface{ SQLRecorder() database.SQLRecorder })
	logger := runlog.Channel(app, channel)
	return orodb.LoggerFunc(func(ctx context.Context, event orodb.LogEvent) {
		recordSQL(recorder, name, dialect, event)
		level := slog.LevelInfo
		if event.Level <= orodb.LogLevelError {
			level = slog.LevelError
		} else if event.Level == orodb.LogLevelWarn {
			level = slog.LevelWarn
		} else if event.Level >= orodb.LogLevelDebug {
			level = slog.LevelDebug
		}
		logger.LogAttrs(ctx, level, "sql",
			slog.String("db", name),
			slog.String("dialect", dialect),
			slog.String("sql", event.SQL),
			slog.Int64("rows", event.Rows),
			slog.Int64("duration_ms", event.Duration.Milliseconds()),
			slog.Any("err", event.Err),
		)
	})
}

func recordSQL(recorder interface{ SQLRecorder() database.SQLRecorder }, name string, dialect string, event orodb.LogEvent) {
	if recorder == nil || recorder.SQLRecorder() == nil {
		return
	}
	item := database.SQLLog{
		Time:      core.Now(),
		Database:  name,
		Dialect:   dialect,
		Operation: event.Operation,
		Model:     event.Model,
		Table:     event.Table,
		SQL:       event.SQL,
		Rows:      event.Rows,
		Latency:   event.Duration,
		Slow:      event.Slow,
	}
	if event.Err != nil {
		item.Error = event.Err.Error()
	}
	recorder.SQLRecorder().RecordSQL(item)
}

// OroDatabase is a Runa database runtime for Oro.
type OroDatabase struct {
	name    string
	dialect string
	db      *orodb.DB
	ownsDB  bool
	meta    core.Map
}

// Name returns runtime name.
func (db *OroDatabase) Name() string { return db.name }

// Kind returns runtime kind.
func (db *OroDatabase) Kind() string { return "oro" }

// Raw returns the raw Oro DB.
func (db *OroDatabase) Raw() any { return db.db }

// Oro returns the raw Oro DB.
func (db *OroDatabase) Oro() *orodb.DB { return db.db }

// Ping checks database availability.
func (db *OroDatabase) Ping(ctx context.Context) error {
	if db.db == nil {
		return fmt.Errorf("oro database %s is not open", db.name)
	}
	_, err := db.db.Raw("select 1").Exec(ctx)
	return err
}

// Close closes the database.
func (db *OroDatabase) Close(ctx context.Context) error {
	if db.db == nil || !db.ownsDB {
		return nil
	}
	return db.db.Close(ctx)
}

// Info returns runtime info.
func (db *OroDatabase) Info() database.Info {
	return database.Info{Name: db.name, Kind: db.Kind(), Dialect: db.dialect, Meta: db.meta}
}
