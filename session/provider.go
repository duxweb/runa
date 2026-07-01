package session

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"log/slog"
	"os"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	runlog "github.com/duxweb/runa/log"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

type provider struct {
	runaprovider.Base
	drivers  map[string]Driver
	sessions map[string][]Option
	cookies  []CookieOption
}

func Provider(options ...ProviderOption) runaprovider.Provider {
	item := &provider{drivers: make(map[string]Driver), sessions: make(map[string][]Option)}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (provider *provider) Name() string { return "session" }

func (provider *provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return New(), nil })
	return nil
}

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	if len(provider.cookies) > 0 {
		registry.Cookie(provider.cookies...)
	}
	for name, driver := range provider.drivers {
		registry.RegisterDriver(name, driver)
	}
	for name, options := range provider.sessions {
		registry.Session(name, options...)
	}
	store, err := runaprovider.Invoke[*config.Store](ctx)
	if err != nil {
		return err
	}
	registry.Cookie(securityCookieOptions(ctx, store)...)
	registry.Config(store)
	return ctx.RegisterRouteService(registry)
}

type ProviderOption func(*provider)

func RegisterDriver(name string, driver Driver) ProviderOption {
	return func(provider *provider) {
		if name != "" && driver != nil {
			provider.drivers[name] = driver
		}
	}
}

func RegisterSession(name string, options ...Option) ProviderOption {
	return func(provider *provider) {
		if name != "" {
			provider.sessions[name] = append([]Option(nil), options...)
		}
	}
}

func Cookie(options ...CookieOption) ProviderOption {
	return func(provider *provider) {
		provider.cookies = append(provider.cookies, options...)
	}
}

func securityCookieOptions(ctx runaprovider.Context, store *config.Store) []CookieOption {
	if store == nil {
		return nil
	}
	secret := store.Get[string]("app.secret")
	if secret == "" {
		secret = store.Get[string]("runa.secret")
	}
	if secret == "" {
		secret = os.Getenv("RUNA_SECRET")
	}
	if secret == "" {
		secret = os.Getenv("APP_SECRET")
	}
	options := make([]CookieOption, 0, 3)
	if appEnv(ctx) == "prod" || appEnv(ctx) == "production" {
		options = append(options, Secure(true))
	}
	if secret != "" {
		options = append(options,
			SignKey(deriveKey(secret, "session.sign")),
			EncryptKey(deriveKey(secret, "session.encrypt")),
		)
	} else if !isLocalEnv(appEnv(ctx)) {
		runlog.Channel(ctx, runlog.Error).Warn(
			"app secret is not configured; session keys are process-local and will change after restart",
			slog.String("env", appEnv(ctx)),
		)
	}
	return options
}

func deriveKey(secret string, purpose string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(purpose))
	return mac.Sum(nil)
}

func appEnv(ctx runaprovider.Context) string {
	if app, ok := ctx.App().(interface{ Env() string }); ok {
		return app.Env()
	}
	return ""
}

func isLocalEnv(env string) bool {
	switch env {
	case "", "local", "dev", "development", "test", "testing":
		return true
	default:
		return false
	}
}

type sessionConfig struct {
	Driver       string        `toml:"driver"`
	CookieName   string        `toml:"cookie_name"`
	CookieDomain string        `toml:"cookie_domain"`
	CookiePath   string        `toml:"cookie_path"`
	TTL          time.Duration `toml:"ttl"`
	IdleTimeout  time.Duration `toml:"idle_timeout"`
	Shared       bool          `toml:"shared"`
	Meta         core.Map      `toml:"meta"`
}

func configOptions(store *config.Store, name string) []Option {
	var item sessionConfig
	ok, err := config.BindNamed(store, "session", "sessions", name, &item)
	if err != nil || !ok {
		return nil
	}
	options := make([]Option, 0, 8+len(item.Meta))
	if item.Driver != "" {
		options = append(options, Use(item.Driver))
	}
	if item.CookieName != "" {
		options = append(options, CookieName(item.CookieName))
	}
	if item.CookieDomain != "" {
		options = append(options, CookieDomain(item.CookieDomain))
	}
	if item.CookiePath != "" {
		options = append(options, CookiePath(item.CookiePath))
	}
	if item.TTL > 0 {
		options = append(options, TTL(item.TTL))
	}
	if item.IdleTimeout > 0 {
		options = append(options, IdleTimeout(item.IdleTimeout))
	}
	if item.Shared {
		options = append(options, Shared(true))
	}
	for key, value := range item.Meta {
		options = append(options, Meta(key, value))
	}
	return options
}
