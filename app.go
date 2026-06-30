package runa

import (
	"context"
	"io"

	"github.com/duxweb/runa/runtime"
)

// App is the Runa application facade type.
type App = runtime.App

// New creates a Runa application.
func New(options ...runtime.Option) *runtime.App {
	return runtime.New(options...)
}

// BasePath sets the application base directory.
func BasePath(path string) runtime.Option { return runtime.BasePath(path) }

// AppPath sets the application source directory relative to the base path.
func AppPath(path string) runtime.Option { return runtime.AppPath(path) }

// ConfigPath sets the configuration directory relative to the base path.
func ConfigPath(path string) runtime.Option { return runtime.ConfigPath(path) }

// DataPath sets the writable data directory relative to the base path.
func DataPath(path string) runtime.Option { return runtime.DataPath(path) }

// PublicPath sets the public asset directory relative to the base path.
func PublicPath(path string) runtime.Option { return runtime.PublicPath(path) }

// Env sets the application environment name.
func Env(name string) runtime.Option { return runtime.Env(name) }

// Timezone sets the application timezone.
func Timezone(name string) runtime.Option { return runtime.Timezone(name) }

// Writer overrides the standard and error output writers.
func Writer(out io.Writer, errOut ...io.Writer) runtime.Option {
	return runtime.Writer(out, errOut...)
}

// DefaultContext returns the framework root context.
func DefaultContext() context.Context { return runtime.DefaultContext() }
