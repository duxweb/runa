package redis

import (
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Option configures Redis rate provider connection and driver registration.
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
}

func Client(client *goredis.Client) Option { return func(options *options) { options.client = client } }
func Addr(value string) Option             { return func(options *options) { options.addr = value } }
func Auth(username string, password string) Option {
	return func(options *options) { options.username, options.password = username, password }
}
func Password(value string) Option { return func(options *options) { options.password = value } }
func DB(value int) Option          { return func(options *options) { options.db = value } }
func DialTimeout(value time.Duration) Option {
	return func(options *options) { options.dialTimeout = value }
}
func ReadTimeout(value time.Duration) Option {
	return func(options *options) { options.readTimeout = value }
}
func WriteTimeout(value time.Duration) Option {
	return func(options *options) { options.writeTimeout = value }
}
func PoolSize(value int) Option  { return func(options *options) { options.poolSize = value } }
func MinIdle(value int) Option   { return func(options *options) { options.minIdle = value } }
func Prefix(value string) Option { return func(options *options) { options.prefix = value } }
func Config(path string) Option  { return func(options *options) { options.configPath = path } }
func Use(name string) Option     { return func(options *options) { options.useName = name } }
func Name(value string) Option   { return func(options *options) { options.driverName = value } }
