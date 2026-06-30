package recover

import (
	"runtime/debug"

	"github.com/duxweb/runa/route"
)

// Config configures recover middleware.
type Config struct {
	Next      func(*route.Context) bool
	Stack     bool
	OnRecover func(ctx *route.Context, value any, stack []byte) error
}

// New creates recover middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) (err error) {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			defer func() {
				if value := recover(); value != nil {
					var stack []byte
					if config.Stack {
						stack = debug.Stack()
					}
					if config.OnRecover != nil {
						err = config.OnRecover(ctx, value, stack)
						return
					}
					err = route.PanicError(value, config.Stack)
				}
			}()
			return next(ctx)
		}
	}
}

func firstConfig(configs ...Config) Config {
	config := Config{Stack: true}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		config.Stack = provided.Stack
		if provided.OnRecover != nil {
			config.OnRecover = provided.OnRecover
		}
	}
	return config
}
