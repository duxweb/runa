package oro

import (
	"time"

	orodb "github.com/duxweb/oro"
	"github.com/duxweb/runa/core"
)

// Option configures an Oro database driver.
type Option func(*options)

type options struct {
	dsn         string
	configPath  string
	dialect     string
	logger      string
	location    *time.Location
	maxOpen     int
	maxIdle     int
	maxLifetime time.Duration
	debug       bool
	meta        core.Map
	db          *orodb.DB
}

// DB uses an existing Oro DB. The runtime will not close injected DBs.
func DB(value *orodb.DB) Option {
	return func(options *options) { options.db = value }
}

// DSN sets the database DSN.
func DSN(value string) Option {
	return func(options *options) { options.dsn = value }
}

// Config reads database config from app config path.
func Config(path string) Option {
	return func(options *options) { options.configPath = path }
}

// Dialect sets the SQL dialect.
func Dialect(name string) Option {
	return func(options *options) { options.dialect = name }
}

// Logger sets the Runa logger channel.
func Logger(name string) Option {
	return func(options *options) { options.logger = name }
}

// Location sets the timezone used by Oro for time values.
func Location(value *time.Location) Option {
	return func(options *options) { options.location = value }
}

// MaxOpen sets max open connections.
func MaxOpen(value int) Option {
	return func(options *options) { options.maxOpen = value }
}

// MaxIdle sets max idle connections.
func MaxIdle(value int) Option {
	return func(options *options) { options.maxIdle = value }
}

// MaxLifetime sets max connection lifetime.
func MaxLifetime(value time.Duration) Option {
	return func(options *options) { options.maxLifetime = value }
}

// Debug enables debug SQL logs.
func Debug(enabled bool) Option {
	return func(options *options) { options.debug = enabled }
}

// Meta sets runtime metadata.
func Meta(key string, value any) Option {
	return func(options *options) {
		if options.meta == nil {
			options.meta = make(core.Map)
		}
		options.meta[key] = value
	}
}
