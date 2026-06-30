package cluster

import (
	"os"
	"time"

	"github.com/duxweb/runa/core"
)

const (
	DefaultService           = "runa"
	DefaultHeartbeatInterval = 10 * time.Second
	DefaultTTL               = 30 * time.Second
)

// Option configures the cluster provider.
type Option func(*options)

type options struct {
	driver            Driver
	driverName        string
	prefix            string
	id                string
	service           string
	env               string
	version           string
	addr              string
	heartbeatInterval time.Duration
	ttl               time.Duration
	meta              core.Map
}

func defaultOptions() options {
	return options{
		driver:            MemoryDriver(),
		driverName:        "memory",
		id:                os.Getenv("RUNA_INSTANCE_ID"),
		service:           DefaultService,
		heartbeatInterval: DefaultHeartbeatInterval,
		ttl:               DefaultTTL,
		meta:              make(core.Map),
	}
}

type fileConfig struct {
	Driver            string        `toml:"driver"`
	Prefix            string        `toml:"prefix"`
	ID                string        `toml:"id"`
	Service           string        `toml:"service"`
	Env               string        `toml:"env"`
	Version           string        `toml:"version"`
	Addr              string        `toml:"addr"`
	HeartbeatInterval time.Duration `toml:"heartbeat_interval"`
	TTL               time.Duration `toml:"ttl"`
	Meta              core.Map      `toml:"meta"`
}

// DriverWith sets the cluster state driver.
func DriverWith(driver Driver) Option {
	return func(options *options) {
		if driver != nil {
			options.driver = driver
			options.driverName = driver.Name()
		}
	}
}

// ID sets the instance id.
func ID(value string) Option {
	return func(options *options) { options.id = value }
}

// UseDriver selects a configured cluster driver by name.
func UseDriver(name string) Option {
	return func(options *options) {
		if name != "" {
			options.driverName = name
			if options.driver != nil && options.driver.Name() != name {
				options.driver = nil
			}
		}
	}
}

// Service sets the service name.
func Service(value string) Option {
	return func(options *options) { options.service = value }
}

// Env sets the environment name.
func Env(value string) Option {
	return func(options *options) { options.env = value }
}

// Version sets the application version.
func Version(value string) Option {
	return func(options *options) { options.version = value }
}

// Addr sets the public or internal instance address.
func Addr(value string) Option {
	return func(options *options) { options.addr = value }
}

// HeartbeatInterval sets heartbeat interval.
func HeartbeatInterval(value time.Duration) Option {
	return func(options *options) {
		if value > 0 {
			options.heartbeatInterval = value
		}
	}
}

// TTL sets stale instance timeout.
func TTL(value time.Duration) Option {
	return func(options *options) {
		if value > 0 {
			options.ttl = value
		}
	}
}

// Meta sets instance metadata.
func Meta(key string, value any) Option {
	return func(options *options) {
		if options.meta == nil {
			options.meta = make(core.Map)
		}
		options.meta[key] = value
	}
}
