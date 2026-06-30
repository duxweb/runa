package console

import "time"

// Config configures console.
type Config struct {
	Title          string        `toml:"title"`
	Mount          string        `toml:"mount"`
	Auth           []string      `toml:"auth"`
	Interval       time.Duration `toml:"interval"`
	SlowThreshold  time.Duration `toml:"slow_threshold"`
	CollectHTTP    bool          `toml:"collect_http"`
	SampleInterval time.Duration `toml:"sample_interval"`
	Panels         []Panel       `toml:"-"`
	Store          MonitorStore  `toml:"-"`
}

// Option configures console provider.
type Option func(*Config)

func defaultConfig() Config {
	return Config{Title: "Runa Console", Interval: 5 * time.Second, SlowThreshold: 500 * time.Millisecond, CollectHTTP: true, SampleInterval: 5 * time.Second}
}

// Title sets console title.
func Title(value string) Option { return func(config *Config) { config.Title = value } }

// MountAt sets provider auto mount path.
func MountAt(path string) Option { return func(config *Config) { config.Mount = path } }

// Auth protects console routes with named Runa authenticators.
func Auth(names ...string) Option {
	return func(config *Config) { config.Auth = append([]string(nil), names...) }
}

// Interval sets frontend refresh interval.
func Interval(value time.Duration) Option {
	return func(config *Config) {
		if value > 0 {
			config.Interval = value
		}
	}
}

// SlowThreshold sets the request latency threshold used by slow logs.
func SlowThreshold(value time.Duration) Option {
	return func(config *Config) {
		if value > 0 {
			config.SlowThreshold = value
		}
	}
}

// CollectHTTP enables or disables console HTTP monitoring.
func CollectHTTP(value bool) Option {
	return func(config *Config) { config.CollectHTTP = value }
}

// SampleInterval sets console background metric sampling interval.
func SampleInterval(value time.Duration) Option {
	return func(config *Config) {
		if value > 0 {
			config.SampleInterval = value
		}
	}
}

// Panels registers console extension panels.
func Panels(items ...Panel) Option {
	return func(config *Config) {
		config.Panels = append(config.Panels, items...)
	}
}

// Store sets the monitor store used by console metrics and logs.
func Store(store MonitorStore) Option {
	return func(config *Config) { config.Store = store }
}
