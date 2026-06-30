package runtime

import (
	"fmt"

	runacommand "github.com/duxweb/runa/command"
	"github.com/duxweb/runa/host"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

var _ runaprovider.Context = (*App)(nil)

// App exposes the concrete application object to provider.Context users.
func (app *App) App() any { return app }

// Injector exposes the application DI container to providers.
func (app *App) Injector() do.Injector { return app.container }

// RegisterCommand registers commands from provider implementations.
func (app *App) RegisterCommand(commands ...runacommand.Command) error {
	app.Command(commands...)
	return nil
}

// RegisterService registers services from provider implementations.
func (app *App) RegisterService(services ...any) error {
	for _, service := range services {
		typed, ok := service.(Service)
		if !ok {
			return fmt.Errorf("provider service %T does not implement runa.Service", service)
		}
		app.Service(typed)
	}
	return nil
}

// RegisterModule registers modules from provider implementations.
func (app *App) RegisterModule(modules ...any) error {
	for _, module := range modules {
		typed, ok := module.(Module)
		if !ok {
			return fmt.Errorf("provider module %T does not implement runa.Module", module)
		}
		app.Module(typed)
	}
	return nil
}

// RegisterHost registers host units from provider implementations.
func (app *App) RegisterHost(units ...host.Unit) error {
	app.mu.Lock()
	manager := app.hosts
	app.mu.Unlock()
	return manager.Register(units...)
}

// RegisterRouteService broadcasts app-scoped services to transport providers
// that opt into route-style service binding.
func (app *App) RegisterRouteService(services ...any) error {
	app.mu.Lock()
	providers := append([]runaprovider.Provider(nil), app.providers...)
	app.mu.Unlock()
	sortProviders(providers)
	for _, provider := range providers {
		if binder, ok := provider.(runaprovider.RouteServiceBinder); ok {
			if err := binder.RegisterRouteService(services...); err != nil {
				return err
			}
		}
	}
	return nil
}
