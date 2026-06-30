package bodylimit

import (
	"fmt"
	"net/http"

	"github.com/duxweb/runa/route"
)

// Config configures body size limit middleware.
type Config struct {
	Next  func(*route.Context) bool
	Limit int64
}

// New creates body size limit middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			if ctx.Request().ContentLength > config.Limit {
				return fmt.Errorf("request body too large")
			}
			ctx.Request().Body = http.MaxBytesReader(ctx.Response(), ctx.Request().Body, config.Limit)
			return next(ctx)
		}
	}
}

func firstConfig(configs ...Config) Config {
	config := Config{Limit: 4 << 20}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		if provided.Limit > 0 {
			config.Limit = provided.Limit
		}
	}
	return config
}
