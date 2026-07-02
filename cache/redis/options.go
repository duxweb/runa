package redis

import (
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Option configures Redis cache provider connection and driver registration.
type Option func(*options)

type options struct {
	driverName   string
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
	prefix       string
	ttl          time.Duration
}

// Client uses an existing Redis client. The driver will not close injected clients.
func Client(client *goredis.Client) Option { return func(options *options) { options.client = client } }

// Addr sets the Redis server address used when the driver creates its own client.
func Addr(value string) Option { return func(options *options) { options.addr = value } }

// Auth sets Redis username and password.
func Auth(username string, password string) Option {
	return func(options *options) { options.username, options.password = username, password }
}

// Password sets Redis password.
func Password(value string) Option { return func(options *options) { options.password = value } }

// DB sets Redis logical database index.
func DB(value int) Option { return func(options *options) { options.db = value } }

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
func PoolSize(value int) Option { return func(options *options) { options.poolSize = value } }

// MinIdle sets Redis minimum idle connections.
func MinIdle(value int) Option { return func(options *options) { options.minIdle = value } }

// Prefix sets Redis cache key prefix.
func Prefix(value string) Option { return func(options *options) { options.prefix = value } }

// TTL sets default cache TTL.
func TTL(value time.Duration) Option { return func(options *options) { options.ttl = value } }

// Config sets the feature-specific config path. Defaults to cache.redis.
func Config(path string) Option { return func(options *options) { options.configPath = path } }

// Use selects the shared redis config name used by Provider.
func Use(name string) Option { return func(options *options) { options.useName = name } }

// Name sets the cache driver registration name used by Provider.
func Name(value string) Option { return func(options *options) { options.driverName = value } }
