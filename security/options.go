package security

import (
	"strings"
	"time"

	"github.com/duxweb/runa/route"
)

// Development applies development-friendly defaults.
func Development() Option {
	return func(config *Config) {
		config.Env = "development"
		config.RecoverStack = true
		config.TimeoutValue = 120 * time.Second
		config.TrustedProxies = []string{"127.0.0.1", "::1"}
	}
}

// Production applies production-friendly defaults.
func Production() Option {
	return func(config *Config) {
		config.Env = "production"
		config.RecoverStack = false
		config.TimeoutValue = 30 * time.Second
		config.TrustedProxies = nil
	}
}

// Next skips the security chain for matching requests.
func Next(fn func(ctx *route.Context) bool) Option {
	return func(config *Config) { config.Next = fn }
}

// BodyLimit sets request body limit.
func BodyLimit(value string) Option {
	return func(config *Config) { config.BodyLimitValue = value }
}

// Timeout sets request timeout.
func Timeout(value time.Duration) Option {
	return func(config *Config) { config.TimeoutValue = value }
}

// TrustedProxies sets trusted proxy list for real ip parsing.
func TrustedProxies(values ...string) Option {
	return func(config *Config) { config.TrustedProxies = append([]string(nil), values...) }
}

// SkipPaths excludes matching paths from access logging.
func SkipPaths(values ...string) Option {
	return func(config *Config) { config.SkipPaths = append([]string(nil), values...) }
}

// Disable disables named middlewares.
func Disable(names ...string) Option {
	return func(config *Config) {
		for _, name := range names {
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "recover":
				config.Recover = false
			case "requestid", "request_id":
				config.RequestID = false
			case "realip", "real_ip":
				config.RealIP = false
			case "logger", "log":
				config.Logger = false
			case "bodylimit", "body_limit":
				config.BodyLimit = false
			case "timeout":
				config.Timeout = false
			case "helmet":
				config.Helmet = false
			}
		}
	}
}
