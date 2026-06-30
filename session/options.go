package session

import (
	"crypto/rand"
	"net/http"
	"strings"
	"time"

	"github.com/duxweb/runa/core"
)

// Option configures a named session pool.
type Option interface{ ApplySession(*Options) }

// CookieOption configures cookie defaults or one cookie write.
type CookieOption interface{ ApplyCookie(*CookieOptions) }

// DriverOption configures session stores.
type DriverOption interface{ ApplyDriver(*DriverOptions) }

// Options stores session pool configuration.
type Options struct {
	Driver      string
	CookieName  string
	Cookie      CookieOptions
	TTL         time.Duration
	IdleTimeout time.Duration
	Shared      bool
	Meta        core.Map
}

// CookieOptions stores HTTP cookie configuration.
type CookieOptions struct {
	SignKey     []byte
	EncryptKey  []byte
	Domain      string
	Path        string
	MaxAge      int
	Expires     time.Time
	HTTPOnly    bool
	Secure      bool
	SameSite    http.SameSite
	Partitioned bool
}

// DriverOptions stores driver configuration.
type DriverOptions struct {
	Name   string
	Prefix string
	TTL    time.Duration
	Meta   core.Map
}

type optionFunc func(*Options)
type cookieOptionFunc func(*CookieOptions)
type driverOptionFunc func(*DriverOptions)

func (fn optionFunc) ApplySession(options *Options)            { fn(options) }
func (fn cookieOptionFunc) ApplyCookie(options *CookieOptions) { fn(options) }
func (fn driverOptionFunc) ApplyDriver(options *DriverOptions) { fn(options) }

var defaultCookieKeys = newDefaultCookieKeys()

type cookieKeys struct {
	sign    []byte
	encrypt []byte
}

func newDefaultCookieKeys() cookieKeys {
	return cookieKeys{
		sign:    randomKey(),
		encrypt: randomKey(),
	}
}

func randomKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return key
}

// Driver selects the session storage driver.
func Use(name string) Option {
	return optionFunc(func(options *Options) { options.Driver = name })
}

// CookieName sets the session id cookie name.
func CookieName(name string) Option {
	return optionFunc(func(options *Options) { options.CookieName = name })
}

// CookieDomain sets the session cookie domain.
func CookieDomain(domain string) Option {
	return optionFunc(func(options *Options) { options.Cookie.Domain = domain })
}

// CookiePath sets the session cookie path.
func CookiePath(path string) Option {
	return optionFunc(func(options *Options) { options.Cookie.Path = path })
}

// TTL sets the maximum session lifetime.
func TTL(duration time.Duration) Option {
	return optionFunc(func(options *Options) { options.TTL = duration })
}

// IdleTimeout sets session idle lifetime metadata.
func IdleTimeout(duration time.Duration) Option {
	return optionFunc(func(options *Options) { options.IdleTimeout = duration })
}

// Shared allows a cookie to be shared across projects/subdomains.
func Shared(enabled bool) Option {
	return optionFunc(func(options *Options) { options.Shared = enabled })
}

// Meta stores arbitrary session pool metadata.
func Meta(key string, value any) Option {
	return optionFunc(func(options *Options) {
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	})
}

// SignKey sets the cookie signing key.
func SignKey(key []byte) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.SignKey = append([]byte(nil), key...) })
}

// EncryptKey sets the cookie encryption key.
func EncryptKey(key []byte) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.EncryptKey = append([]byte(nil), key...) })
}

// Domain sets cookie domain.
func Domain(domain string) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.Domain = domain })
}

// Path sets cookie path.
func Path(path string) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.Path = path })
}

// MaxAge sets cookie max-age.
func MaxAge(seconds int) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.MaxAge = seconds })
}

// Expires sets cookie expiration time.
func Expires(value time.Time) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.Expires = value })
}

// HTTPOnly sets cookie HttpOnly.
func HTTPOnly(enabled bool) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.HTTPOnly = enabled })
}

// Secure sets cookie Secure.
func Secure(enabled bool) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.Secure = enabled })
}

// SameSite sets cookie SameSite.
func SameSite(mode http.SameSite) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.SameSite = mode })
}

// Partitioned sets cookie Partitioned.
func Partitioned(enabled bool) CookieOption {
	return cookieOptionFunc(func(options *CookieOptions) { options.Partitioned = enabled })
}

// Name sets store name metadata.
func Name(value string) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) { options.Name = value })
}

// Prefix sets store key prefix.
func Prefix(value string) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) { options.Prefix = value })
}

// DriverTTL sets driver default ttl.
func DriverTTL(value time.Duration) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) { options.TTL = value })
}

// DriverMeta stores arbitrary store metadata.
func DriverMeta(key string, value any) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) {
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	})
}

func DefaultCookieOptions() CookieOptions {
	return CookieOptions{
		SignKey:    append([]byte(nil), defaultCookieKeys.sign...),
		EncryptKey: append([]byte(nil), defaultCookieKeys.encrypt...),
		Path:       "/",
		HTTPOnly:   true,
		SameSite:   http.SameSiteLaxMode,
	}
}

// ApplyCookieOptions applies cookie options to a base config.
func ApplyCookieOptions(base CookieOptions, options ...CookieOption) CookieOptions {
	return applyCookieOptions(base, options...)
}

func applyOptions(defaultCookie CookieOptions, options ...Option) Options {
	opts := Options{Driver: DriverMemory, CookieName: DefaultCookieName, Cookie: normalizeCookie(defaultCookie), TTL: DefaultTTL, Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplySession(&opts)
		}
	}
	if opts.Driver == "" {
		opts.Driver = DriverMemory
	}
	if opts.CookieName == "" {
		opts.CookieName = DefaultCookieName
	}
	if opts.TTL <= 0 {
		opts.TTL = DefaultTTL
	}
	opts.Cookie = normalizeCookieForName(opts.CookieName, opts.Cookie)
	return opts
}

func applyCookieOptions(base CookieOptions, options ...CookieOption) CookieOptions {
	opts := base
	for _, option := range options {
		if option != nil {
			option.ApplyCookie(&opts)
		}
	}
	return normalizeCookie(opts)
}

func applyDriverOptions(options ...DriverOption) DriverOptions {
	opts := DriverOptions{Name: DriverMemory, Prefix: "runa:session:", TTL: DefaultTTL, Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyDriver(&opts)
		}
	}
	if opts.Name == "" {
		opts.Name = DriverMemory
	}
	if opts.Prefix == "" {
		opts.Prefix = "runa:session:"
	}
	if opts.TTL <= 0 {
		opts.TTL = DefaultTTL
	}
	return opts
}

func normalizeCookie(options CookieOptions) CookieOptions {
	if len(options.SignKey) == 0 {
		options.SignKey = append([]byte(nil), defaultCookieKeys.sign...)
	}
	if len(options.EncryptKey) == 0 {
		options.EncryptKey = append([]byte(nil), defaultCookieKeys.encrypt...)
	}
	if options.Path == "" {
		options.Path = "/"
	}
	if options.SameSite == 0 {
		options.SameSite = http.SameSiteLaxMode
	}
	return options
}

func normalizeCookieForName(name string, options CookieOptions) CookieOptions {
	options = normalizeCookie(options)
	if strings.HasPrefix(name, "__Secure-") {
		options.Secure = true
	}
	if strings.HasPrefix(name, "__Host-") {
		options.Secure = true
		options.Path = "/"
		options.Domain = ""
	}
	return options
}
