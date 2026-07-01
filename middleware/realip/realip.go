package realip

import (
	"net"
	"strings"

	"github.com/duxweb/runa/route"
)

// Config configures real ip middleware.
type Config struct {
	Next           func(*route.Context) bool
	TrustedProxies []string
	Headers        []string
	SchemeHeaders  []string
	HostHeaders    []string
}

// New creates real ip middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	trusted := parseTrusted(config.TrustedProxies)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			if trustedIP(ctx.IP(), trusted) {
				if ip := forwardedIP(ctx, config.Headers, trusted); ip != "" {
					ctx.Locals(route.LocalIP, ip)
				}
				if scheme := firstHeader(ctx, config.SchemeHeaders); scheme != "" {
					ctx.Locals(route.LocalScheme, scheme)
				}
				if host := firstHeader(ctx, config.HostHeaders); host != "" {
					ctx.Locals(route.LocalHost, host)
				}
			}
			return next(ctx)
		}
	}
}

func firstConfig(configs ...Config) Config {
	config := Config{
		TrustedProxies: []string{"127.0.0.1", "::1"},
		Headers:        []string{"X-Forwarded-For", "X-Real-IP", "CF-Connecting-IP"},
		SchemeHeaders:  []string{"X-Forwarded-Proto"},
		HostHeaders:    []string{"X-Forwarded-Host"},
	}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		if provided.TrustedProxies != nil {
			config.TrustedProxies = provided.TrustedProxies
		}
		if provided.Headers != nil {
			config.Headers = provided.Headers
		}
		if provided.SchemeHeaders != nil {
			config.SchemeHeaders = provided.SchemeHeaders
		}
		if provided.HostHeaders != nil {
			config.HostHeaders = provided.HostHeaders
		}
	}
	return config
}

func parseTrusted(values []string) []netipMatcher {
	matchers := make([]netipMatcher, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(value); err == nil {
			matchers = append(matchers, netipMatcher{network: network})
			continue
		}
		if ip := net.ParseIP(value); ip != nil {
			matchers = append(matchers, netipMatcher{ip: ip})
		}
	}
	return matchers
}

type netipMatcher struct {
	ip      net.IP
	network *net.IPNet
}

func trustedIP(value string, matchers []netipMatcher) bool {
	ip := net.ParseIP(value)
	if ip == nil {
		return false
	}
	for _, matcher := range matchers {
		if matcher.network != nil && matcher.network.Contains(ip) {
			return true
		}
		if matcher.ip != nil && matcher.ip.Equal(ip) {
			return true
		}
	}
	return false
}

func forwardedIP(ctx *route.Context, headers []string, trusted []netipMatcher) string {
	for _, header := range headers {
		value := ctx.Header[string](header)
		if value == "" {
			continue
		}
		parts := strings.Split(value, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			if ip != "" && !trustedIP(ip, trusted) {
				return ip
			}
		}
	}
	return ""
}

func firstHeader(ctx *route.Context, headers []string) string {
	for _, header := range headers {
		value := ctx.Header[string](header)
		if value != "" {
			return value
		}
	}
	return ""
}
