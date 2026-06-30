package healthcheck

import (
	"net/http"

	"github.com/duxweb/runa/route"
)

// Config configures healthcheck middleware.
type Config struct {
	Next    func(*route.Context) bool
	Path    string
	Status  int
	Message string
}

// New creates healthcheck middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			if ctx.Request().URL.Path == config.Path {
				return ctx.Status(config.Status).Text(config.Message)
			}
			return next(ctx)
		}
	}
}

func firstConfig(configs ...Config) Config {
	config := Config{Path: "/health", Status: http.StatusOK, Message: "ok"}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		if provided.Path != "" {
			config.Path = provided.Path
		}
		if provided.Status > 0 {
			config.Status = provided.Status
		}
		if provided.Message != "" {
			config.Message = provided.Message
		}
	}
	return config
}
