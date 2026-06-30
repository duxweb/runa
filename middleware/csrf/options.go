package csrf

import (
	"net/http"

	"github.com/duxweb/runa/route"
)

// Config configures CSRF middleware.
type Config struct {
	Next         func(*route.Context) bool
	CookieName   string
	HeaderName   string
	FormField    string
	CookiePath   string
	CookieDomain string
	Secure       bool
	HTTPOnly     bool
	SameSite     http.SameSite
	MaxAge       int
	SkipPaths    []string
	OnError      func(*route.Context) error
}

// Option configures CSRF middleware.
type Option func(*Config)

// Next skips CSRF middleware when fn returns true.
func Next(fn func(*route.Context) bool) Option {
	return func(config *Config) { config.Next = fn }
}

// Cookie sets the CSRF cookie name.
func Cookie(name string) Option {
	return func(config *Config) { config.CookieName = name }
}

// Header sets the request header used to submit the CSRF token.
func Header(name string) Option {
	return func(config *Config) { config.HeaderName = name }
}

// Field sets the form field used to submit the CSRF token.
func Field(name string) Option {
	return func(config *Config) { config.FormField = name }
}

// Path sets the CSRF cookie path.
func Path(path string) Option {
	return func(config *Config) { config.CookiePath = path }
}

// Domain sets the CSRF cookie domain.
func Domain(domain string) Option {
	return func(config *Config) { config.CookieDomain = domain }
}

// Secure sets the CSRF cookie Secure flag.
func Secure(value bool) Option {
	return func(config *Config) { config.Secure = value }
}

// HTTPOnly sets the CSRF cookie HttpOnly flag.
func HTTPOnly(value bool) Option {
	return func(config *Config) { config.HTTPOnly = value }
}

// SameSite sets the CSRF cookie SameSite mode.
func SameSite(mode http.SameSite) Option {
	return func(config *Config) { config.SameSite = mode }
}

// MaxAge sets the CSRF cookie Max-Age in seconds.
func MaxAge(seconds int) Option {
	return func(config *Config) { config.MaxAge = seconds }
}

// SkipPaths skips CSRF checks for exact request paths.
func SkipPaths(paths ...string) Option {
	return func(config *Config) { config.SkipPaths = append([]string(nil), paths...) }
}

// OnError sets a custom error handler for rejected requests.
func OnError(fn func(*route.Context) error) Option {
	return func(config *Config) { config.OnError = fn }
}

func applyOptions(options ...Option) Config {
	config := Config{
		CookieName: "runa_csrf_token",
		HeaderName: "X-CSRF-Token",
		FormField:  "_csrf",
		CookiePath: "/",
		SameSite:   http.SameSiteLaxMode,
	}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if config.CookieName == "" {
		config.CookieName = "runa_csrf_token"
	}
	if config.HeaderName == "" {
		config.HeaderName = "X-CSRF-Token"
	}
	if config.FormField == "" {
		config.FormField = "_csrf"
	}
	if config.CookiePath == "" {
		config.CookiePath = "/"
	}
	if config.SameSite == 0 {
		config.SameSite = http.SameSiteLaxMode
	}
	return config
}
