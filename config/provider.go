package config

import (
	"context"
	"os"

	runaprovider "github.com/duxweb/runa/provider"
)

type provider struct {
	basePath string
	paths    Paths
	dir      string
	env      string
	prefixes []string
}

// Provider registers the application config store and loads config sources.
func Provider(basePath string, paths Paths, dir string, env string, prefixes ...string) runaprovider.Provider {
	return &provider{
		basePath: basePath,
		paths:    paths,
		dir:      dir,
		env:      env,
		prefixes: append([]string(nil), prefixes...),
	}
}

func (provider *provider) Name() string { return "config" }

func (provider *provider) Priority() int { return -1000 }

func (provider *provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideValue(ctx, New(provider.basePath, provider.paths))
	return nil
}

func (provider *provider) Register(ctx runaprovider.Context) error {
	for _, path := range []string{
		provider.paths.DataPath("cache"),
		provider.paths.DataPath("logs"),
		provider.paths.DataPath("tmp"),
		provider.paths.DataPath("uploads"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	store, err := runaprovider.Invoke[*Store](ctx)
	if err != nil {
		return err
	}
	if err := registerCommand(ctx, store); err != nil {
		return err
	}
	if err := LoadFiles(store, provider.dir, provider.env, provider.prefixes...); err != nil {
		return err
	}
	return ctx.RegisterRouteService(store)
}

func (provider *provider) Boot(context.Context, runaprovider.Context) error { return nil }

func (provider *provider) Shutdown(context.Context, runaprovider.Context) error { return nil }
