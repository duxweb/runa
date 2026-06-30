package lang

import (
	"context"
	"path/filepath"

	"github.com/duxweb/runa/config"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

type provider struct {
	runaprovider.Base
	registry *Registry
	options  []Option
}

type providerConfig struct {
	Default string   `toml:"default"`
	Dir     string   `toml:"dir"`
	Files   []string `toml:"files"`
}

// Provider registers the language registry.
func Provider(options ...Option) runaprovider.Provider {
	return &provider{options: append([]Option(nil), options...)}
}

func (provider *provider) Name() string { return "lang" }

func (provider *provider) Init(_ context.Context, ctx runaprovider.Context) error {
	provider.registry = New(provider.options...)
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return provider.registry, nil })
	return nil
}

func (provider *provider) Register(ctx runaprovider.Context) error {
	store, err := runaprovider.Invoke[*config.Store](ctx)
	if err != nil {
		return err
	}
	conf := providerConfig{
		Default: provider.registry.defaults[0],
		Dir:     "lang",
	}
	if err := store.Scope("lang").Bind("", &conf); err != nil {
		return err
	}
	if conf.Default != "" {
		provider.registry.reset(conf.Default)
	}
	for _, file := range conf.Files {
		if err := provider.registry.LoadFile(absConfigPath(ctx, file)); err != nil {
			return err
		}
	}
	dir := absConfigPath(ctx, conf.Dir)
	if dirExists(dir) {
		return provider.registry.LoadDir(dir)
	}
	return nil
}

func absConfigPath(ctx runaprovider.Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if app, ok := ctx.App().(interface{ ConfigPath(...string) string }); ok {
		return app.ConfigPath(path)
	}
	return path
}
