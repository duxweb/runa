package provider

import (
	"context"
	"sync"

	"github.com/duxweb/runa/command"
	"github.com/duxweb/runa/host"
	"github.com/samber/do/v2"
)

var defaultInjector = struct {
	sync.RWMutex
	value do.Injector
}{}

// Context is passed to providers during application freeze.
type Context interface {
	// App returns the concrete application object for packages that intentionally
	// need app-level route, lifecycle, or infrastructure APIs.
	App() any
	// Injector returns the application DI container.
	Injector() do.Injector
	// RegisterCommand registers CLI commands without depending on the root package.
	RegisterCommand(commands ...command.Command) error
	// RegisterService registers lifecycle services without depending on the root package.
	RegisterService(services ...any) error
	// RegisterModule registers business modules without depending on the root package.
	RegisterModule(modules ...any) error
	// RegisterHost registers host units without depending on the root package.
	RegisterHost(units ...host.Unit) error
	// RegisterRouteService exposes app-scoped services to route contexts without
	// making the runtime package depend on individual subsystem packages.
	RegisterRouteService(services ...any) error
}

// Provider is the extension contract managed by the application lifecycle.
type Provider interface {
	Name() string
	Init(ctx context.Context, app Context) error
	Register(ctx Context) error
	Boot(ctx context.Context, app Context) error
	Shutdown(ctx context.Context, app Context) error
}

// Resolver runs after all providers, services, and modules have registered
// and before any provider boot hook starts.
type Resolver interface {
	Resolve(ctx context.Context) error
}

// RouteServiceBinder accepts app-scoped services for route-like transports
// without making the runtime package depend on a transport implementation.
type RouteServiceBinder interface {
	RegisterRouteService(services ...any) error
}

// Ordered lets a provider run before or after other providers.
type Ordered interface {
	Priority() int
}

// Base provides no-op lifecycle methods for provider implementations.
type Base struct{}

func (Base) Init(context.Context, Context) error { return nil }
func (Base) Register(Context) error              { return nil }
func (Base) Boot(context.Context, Context) error { return nil }
func (Base) Shutdown(context.Context, Context) error {
	return nil
}
func (Base) Priority() int { return 0 }

// PriorityOf returns a provider's priority. Lower values run earlier.
func PriorityOf(provider Provider) int {
	if ordered, ok := provider.(Ordered); ok {
		return ordered.Priority()
	}
	return 0
}

// Provide registers a lazy dependency constructor on the provider context.
func Provide[T any](ctx Context, constructor func(do.Injector) (T, error)) {
	do.Provide(ctx.Injector(), constructor)
}

// ProvideDefault registers a lazy dependency only when it has not already been registered.
func ProvideDefault[T any](ctx Context, constructor func(do.Injector) (T, error)) {
	name := do.NameOf[T]()
	if hasService(ctx.Injector(), name) {
		return
	}
	do.Provide(ctx.Injector(), constructor)
}

// ProvideValue registers an eager dependency value on the provider context.
func ProvideValue[T any](ctx Context, value T) {
	do.ProvideValue(ctx.Injector(), value)
}

// ProvideValueOnce registers an eager dependency value only when it has not already been registered.
func ProvideValueOnce[T any](ctx Context, value T) {
	ProvideValueOnceTo(ctx.Injector(), value)
}

// ProvideValueOnceTo registers an eager dependency value on an injector only when absent.
func ProvideValueOnceTo[T any](injector do.Injector, value T) {
	name := do.NameOf[T]()
	if hasService(injector, name) {
		return
	}
	do.ProvideValue(injector, value)
}

func hasService(injector do.Injector, name string) bool {
	for _, service := range injector.ListProvidedServices() {
		if service.Service == name {
			return true
		}
	}
	return false
}

// Invoke resolves a dependency from the provider context.
func Invoke[T any](ctx Context) (T, error) {
	return do.Invoke[T](ctx.Injector())
}

// MustInvoke resolves a dependency from the provider context or panics.
func MustInvoke[T any](ctx Context) T {
	return do.MustInvoke[T](ctx.Injector())
}

// SetDefaultInjector sets the process-wide default dependency container.
func SetDefaultInjector(injector do.Injector) {
	defaultInjector.Lock()
	defaultInjector.value = injector
	defaultInjector.Unlock()
}

// DefaultInjector returns the process-wide default dependency container.
func DefaultInjector() do.Injector {
	defaultInjector.RLock()
	injector := defaultInjector.value
	defaultInjector.RUnlock()
	return injector
}

// InvokeDefault resolves a dependency from the default container.
func InvokeDefault[T any]() (T, error) {
	return do.Invoke[T](DefaultInjector())
}

// MustInvokeDefault resolves a dependency from the default container or panics.
func MustInvokeDefault[T any]() T {
	return do.MustInvoke[T](DefaultInjector())
}
