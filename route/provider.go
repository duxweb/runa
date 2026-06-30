package route

import (
	"context"

	"github.com/duxweb/runa/provider"
)

type routeProvider struct {
	provider.Base
	registry *Registry
	server   ServerConfig
	listen   bool
}

// Provider creates the route provider.
func Provider(options ...ProviderOption) provider.Provider {
	item := &routeProvider{registry: SetDefault(New())}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (routeProvider) Name() string { return "route" }

func (routeProvider) Priority() int { return -900 }

func (item *routeProvider) Init(_ context.Context, ctx provider.Context) error {
	item.registry = initDefault(item.registry)
	provideDefault(ctx, item.registry)
	return nil
}

func (item *routeProvider) Register(ctx provider.Context) error {
	if err := ctx.RegisterCommand(ListCommand(item.registry)); err != nil {
		return err
	}
	if item.listen {
		return ctx.RegisterHost(item.registry.Server(item.server))
	}
	return nil
}

func (item *routeProvider) Resolve(context.Context) error {
	return item.registry.ResolveMounts()
}

func (item *routeProvider) RegisterRouteService(services ...any) error {
	for _, service := range services {
		item.registry.Service(service)
	}
	return nil
}

// ProviderOption configures the route provider.
type ProviderOption func(*routeProvider)

// UseRegistry uses an existing route registry.
func UseRegistry(registry *Registry) ProviderOption {
	return func(provider *routeProvider) {
		if registry != nil {
			provider.registry = SetDefault(registry)
		}
	}
}

// Server registers an HTTP host for the route registry.
func Server(config ServerConfig) ProviderOption {
	return func(provider *routeProvider) {
		provider.server = config
		provider.listen = true
	}
}

// Addr registers an HTTP host bound to addr.
func Addr(addr string) ProviderOption {
	return Server(ServerConfig{Addr: addr})
}
