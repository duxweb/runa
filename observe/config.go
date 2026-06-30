package observe

import "time"

// Config configures the observe provider.
type Config struct {
	Service string        `toml:"service"`
	Env     string        `toml:"env"`
	Version string        `toml:"version"`
	Timeout time.Duration `toml:"timeout"`
	Mount   string        `toml:"mount"`
	Debug   bool          `toml:"debug"`
}

// Option configures the observe provider.
type Option func(*state)

func defaultConfig(config Config) Config {
	if config.Timeout <= 0 {
		config.Timeout = 2 * time.Second
	}
	return config
}

// Timeout sets checker timeout.
func Timeout(value time.Duration) Option {
	return func(state *state) { state.config.Timeout = value }
}

// MountAt sets the route prefix for observe endpoints.
func MountAt(prefix string) Option {
	return func(state *state) { state.config.Mount = prefix }
}

// Debug enables debug endpoints.
func Debug() Option {
	return func(state *state) { state.config.Debug = true }
}

// Health registers a health checker.
func Health(name string, checker Checker) Option {
	return func(state *state) { state.health.Add(name, checker) }
}

// Ready registers a ready checker.
func Ready(name string, checker Checker) Option {
	return func(state *state) { state.ready.Add(name, checker) }
}

// Metrics registers a metrics exporter.
func Metrics(exporter Exporter) Option {
	return func(state *state) { state.metrics = exporter }
}

// Trace registers a trace installer.
func Trace(installer Installer) Option {
	return func(state *state) { state.traces = append(state.traces, installer) }
}
