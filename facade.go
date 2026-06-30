package runa

import (
	"context"

	runacommand "github.com/duxweb/runa/command"
	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/host"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/runtime"
)

// Default returns the process-wide default application.
func Default() *App { return runtime.Default() }

// SetDefault sets the process-wide default application.
func SetDefault(app *App) *App { return runtime.SetDefault(app) }

// Install installs providers on the default application.
func Install(providers ...runaprovider.Provider) *App { return Default().Install(providers...) }

// Service registers services on the default application.
func Service(services ...runtime.Service) *App { return Default().Service(services...) }

// Module registers modules on the default application.
func Module(modules ...runtime.Module) *App { return Default().Module(modules...) }

// Command registers commands on the default application.
func Command(commands ...runacommand.Command) *App { return Default().Command(commands...) }

// Host registers host units on the default application.
func Host(units ...host.Unit) *App { return Default().Host(units...) }

// Run executes the default application.
func Run(ctx context.Context) error { return runtime.Run(ctx) }

// Execute runs a command on the default application.
func Execute(ctx context.Context, args []string) error { return Default().Execute(ctx, args) }

// Freeze compiles and boots the default application.
func Freeze(ctx context.Context) error { return Default().Freeze(ctx) }

// Shutdown stops the default application.
func Shutdown(ctx context.Context) error { return Default().Shutdown(ctx) }

// Config returns the default config store or a named config view.
func Config(name ...string) *config.Store {
	store := runtime.MustInvokeDefault[*config.Store]()
	if len(name) > 0 && name[0] != "" {
		return store.Scope(name[0])
	}
	return store
}
