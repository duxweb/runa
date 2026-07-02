package redis

import (
	"time"

	"github.com/duxweb/runa/core"
	goredis "github.com/redis/go-redis/v9"
)

// Option configures a Redis database driver.
type Option func(*options)

type options struct {
	addr         string
	username     string
	password     string
	db           int
	configPath   string
	logger       string
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	poolSize     int
	minIdle      int
	meta         core.Map
	client       *goredis.Client
}

// Client uses an existing Redis client. The runtime will not close injected clients.
func Client(client *goredis.Client) Option {
	return func(options *options) { options.client = client }
}

// Addr sets the Redis server address.
func Addr(value string) Option {
	return func(options *options) { options.addr = value }
}

// Auth sets Redis username and password.
func Auth(username string, password string) Option {
	return func(options *options) {
		options.username = username
		options.password = password
	}
}

// Password sets Redis password.
func Password(value string) Option {
	return func(options *options) { options.password = value }
}

// DB sets Redis logical database index.
func DB(value int) Option {
	return func(options *options) { options.db = value }
}

// Config reads Redis config from app config path.
func Config(path string) Option {
	return func(options *options) { options.configPath = path }
}

// Logger sets the Runa logger channel.
func Logger(name string) Option {
	return func(options *options) { options.logger = name }
}

// DialTimeout sets Redis dial timeout.
func DialTimeout(value time.Duration) Option {
	return func(options *options) { options.dialTimeout = value }
}

// ReadTimeout sets Redis read timeout.
func ReadTimeout(value time.Duration) Option {
	return func(options *options) { options.readTimeout = value }
}

// WriteTimeout sets Redis write timeout.
func WriteTimeout(value time.Duration) Option {
	return func(options *options) { options.writeTimeout = value }
}

// PoolSize sets Redis connection pool size.
func PoolSize(value int) Option {
	return func(options *options) { options.poolSize = value }
}

// MinIdle sets Redis minimum idle connections.
func MinIdle(value int) Option {
	return func(options *options) { options.minIdle = value }
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
