package route

import (
	"context"
	"errors"
	"sync"

	"github.com/duxweb/runa/provider"
)

var defaultRegistry = struct {
	sync.RWMutex
	value *Registry
}{}

// Default returns the process-wide default route registry installed by Provider.
func Default() *Registry {
	defaultRegistry.RLock()
	registry := defaultRegistry.value
	defaultRegistry.RUnlock()
	if registry != nil {
		return registry
	}
	panic("route provider is not installed")
}

// SetDefault sets the process-wide default route registry.
func SetDefault(registry *Registry) *Registry {
	defaultRegistry.Lock()
	defaultRegistry.value = registry
	defaultRegistry.Unlock()
	return registry
}

// Use appends root middlewares to the default route registry.
func Use(middlewares ...Middleware) { Default().Use(middlewares...) }

// Domain registers a domain on the default route registry.
func Domain(name string, prefix string, fn ...func(*Group)) *Group {
	return Default().Domain(name, prefix, fn...)
}

// GetDomain returns a named domain from the default route registry.
func GetDomain(name string) *Group { return Default().GetDomain(name) }

// GetGroup returns a named group from the default route registry.
func GetGroup(name string) *Group { return Default().GetGroup(name) }

// MountDomain mounts routes into a named domain on the default route registry.
func MountDomain(name string, fn func(*Group)) { Default().MountDomain(name, fn) }

// MountGroup mounts routes into a named group on the default route registry.
func MountGroup(name string, fn func(*Group)) { Default().MountGroup(name, fn) }

// Routes returns registered routes from the default route registry.
func Routes() []*Route { return Default().Routes() }

// Listen starts a standalone HTTP server for the default route registry.
func Listen(ctx context.Context, addr string) error {
	server := Default().Server(ServerConfig{Addr: addr})
	if err := server.Start(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	return errors.Join(server.Stop(context.Background()))
}

func initDefault(registry *Registry) *Registry {
	if registry != nil {
		return SetDefault(registry)
	}
	return Default()
}

func provideDefault(ctx provider.Context, registry *Registry) {
	provider.ProvideValueOnce(ctx, registry)
}
