package security

import (
	"strconv"
	"strings"
	"time"

	"github.com/duxweb/runa/middleware/bodylimit"
	"github.com/duxweb/runa/middleware/helmet"
	"github.com/duxweb/runa/middleware/logger"
	"github.com/duxweb/runa/middleware/realip"
	"github.com/duxweb/runa/middleware/recover"
	"github.com/duxweb/runa/middleware/requestid"
	"github.com/duxweb/runa/middleware/timeout"
	"github.com/duxweb/runa/route"
)

// Config configures the security middleware preset.
type Config struct {
	Next func(ctx *route.Context) bool

	Env string

	Recover   bool
	RequestID bool
	RealIP    bool
	Logger    bool
	BodyLimit bool
	Timeout   bool
	Helmet    bool

	RecoverStack   bool
	BodyLimitValue string
	TimeoutValue   time.Duration
	TrustedProxies []string
	SkipPaths      []string
}

// Option configures Config.
type Option func(*Config)

// New creates the default security middleware chain.
func New(options ...Option) route.Middleware {
	config := defaultConfig()
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	middlewares := make([]route.Middleware, 0, 7)
	if config.Recover {
		middlewares = append(middlewares, recover.New(recover.Config{Next: config.Next, Stack: config.RecoverStack}))
	}
	if config.RequestID {
		middlewares = append(middlewares, requestid.New(requestid.Config{Next: config.Next}))
	}
	if config.RealIP {
		middlewares = append(middlewares, realip.New(realip.Config{Next: config.Next, TrustedProxies: config.TrustedProxies}))
	}
	if config.Logger {
		middlewares = append(middlewares, logger.New(logger.Config{Next: config.Next, SkipPaths: config.SkipPaths}))
	}
	if config.BodyLimit {
		middlewares = append(middlewares, bodylimit.New(bodylimit.Config{Next: config.Next, Limit: parseSize(config.BodyLimitValue)}))
	}
	if config.Timeout {
		middlewares = append(middlewares, timeout.New(timeout.Config{Next: config.Next, Timeout: config.TimeoutValue}))
	}
	if config.Helmet {
		middlewares = append(middlewares, helmet.New(helmet.Config{Next: config.Next}))
	}
	return chain(middlewares...)
}

func chain(items ...route.Middleware) route.Middleware {
	return func(next route.Handler) route.Handler {
		handler := next
		for i := len(items) - 1; i >= 0; i-- {
			if items[i] != nil {
				handler = items[i](handler)
			}
		}
		return handler
	}
}

func defaultConfig() Config {
	return Config{
		Recover:        true,
		RequestID:      true,
		RealIP:         true,
		Logger:         true,
		BodyLimit:      true,
		Timeout:        true,
		Helmet:         true,
		RecoverStack:   false,
		BodyLimitValue: "32MB",
		TimeoutValue:   30 * time.Second,
		TrustedProxies: []string{"127.0.0.1", "::1"},
	}
}

func parseSize(value string) int64 {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return 32 << 20
	}
	units := []struct {
		suffix     string
		multiplier int64
	}{
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
		{"G", 1 << 30},
		{"M", 1 << 20},
		{"K", 1 << 10},
		{"B", 1},
	}
	for _, unit := range units {
		suffix := unit.suffix
		if strings.HasSuffix(value, suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, suffix))
			parsed, err := strconv.ParseInt(number, 10, 64)
			if err != nil || parsed <= 0 {
				return 32 << 20
			}
			return parsed * unit.multiplier
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 32 << 20
	}
	return parsed
}
