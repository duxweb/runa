package cors

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/duxweb/runa/route"
)

// Config configures CORS middleware.
type Config struct {
	Next          func(*route.Context) bool
	AllowOrigins  []string
	AllowMethods  []string
	AllowHeaders  []string
	ExposeHeaders []string
	Credentials   bool
	MaxAge        int
}

// New creates CORS middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			origin := ctx.Header[string]("Origin")
			if origin == "" {
				return next(ctx)
			}
			if allowedOrigin(origin, config.AllowOrigins) {
				applyHeaders(ctx, origin, config)
			}
			if ctx.Request().Method == http.MethodOptions && ctx.Header[string]("Access-Control-Request-Method") != "" {
				ctx.Response().WriteHeader(http.StatusNoContent)
				return nil
			}
			return next(ctx)
		}
	}
}

func firstConfig(configs ...Config) Config {
	config := Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type", "X-Requested-With"},
	}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		if provided.AllowOrigins != nil {
			config.AllowOrigins = provided.AllowOrigins
		}
		if provided.AllowMethods != nil {
			config.AllowMethods = provided.AllowMethods
		}
		if provided.AllowHeaders != nil {
			config.AllowHeaders = provided.AllowHeaders
		}
		if provided.ExposeHeaders != nil {
			config.ExposeHeaders = provided.ExposeHeaders
		}
		config.Credentials = provided.Credentials
		config.MaxAge = provided.MaxAge
	}
	return config
}

func allowedOrigin(origin string, allowed []string) bool {
	for _, item := range allowed {
		if item == "*" || item == origin {
			return true
		}
	}
	return false
}

func applyHeaders(ctx *route.Context, origin string, config Config) {
	header := ctx.Response().Header()
	wildcard := slices.Contains(config.AllowOrigins, "*")
	if wildcard && !config.Credentials {
		header.Set("Access-Control-Allow-Origin", "*")
	} else if !wildcard {
		header.Set("Access-Control-Allow-Origin", origin)
		header.Add("Vary", "Origin")
	}
	if len(config.AllowMethods) > 0 {
		header.Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
	}
	if len(config.AllowHeaders) > 0 {
		header.Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
	}
	if len(config.ExposeHeaders) > 0 {
		header.Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
	}
	if config.Credentials && !wildcard {
		header.Set("Access-Control-Allow-Credentials", "true")
	}
	if config.MaxAge > 0 {
		header.Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
	}
}
