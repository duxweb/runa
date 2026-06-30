package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/duxweb/runa/rate"
	"github.com/duxweb/runa/route"
)

// Use checks a named limiter before running the route handler.
func Use(name string) route.Middleware {
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			registry := route.Service[*rate.Registry](ctx)
			if registry == nil {
				return ctx.Error(http.StatusTooManyRequests, "rate registry is not configured")
			}
			limiter, err := registry.Of(name)
			if err != nil {
				return err
			}
			keys := rateKeys(ctx, name)
			result, err := limiter.Allow(ctx.Context(), keys...)
			if err != nil {
				return err
			}
			writeRateHeaders(ctx, result)
			if !result.Allowed {
				return ctx.Error(http.StatusTooManyRequests, "请求太频繁")
			}
			return next(ctx)
		}
	}
}

func rateKeys(ctx *route.Context, name string) []string {
	keys := []string{}
	if registry := route.Service[*rate.Registry](ctx); registry != nil {
		rule, ok := registry.Rule(name)
		if ok {
			for _, source := range rule.Key {
				if source == nil {
					continue
				}
				value := strings.TrimSpace(source.Value(ctx))
				if value != "" {
					keys = append(keys, source.Name()+":"+value)
				}
			}
		}
	}
	if len(keys) == 0 {
		keys = append(keys, "ip:"+ctx.IP())
	}
	return keys
}

func writeRateHeaders(ctx *route.Context, result rate.Result) {
	ctx.Set("RateLimit-Limit", strconv.Itoa(result.Limit))
	ctx.Set("RateLimit-Remaining", strconv.Itoa(result.Remaining))
	if !result.ResetAt.IsZero() {
		ctx.Set("RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
	}
	if result.RetryAfter > 0 {
		ctx.Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Round(time.Second).Seconds())))
	}
}
