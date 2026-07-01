package log

import (
	"log/slog"

	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

type injectorProvider interface {
	Injector() do.Injector
}

type logProvider interface {
	Log() *Registry
}

// Channel returns a named logger from the supplied app, provider context,
// registry, or the process default registry.
func Channel(app any, name string) *slog.Logger {
	if name == "" {
		name = DefaultName
	}
	if registry := registryOf(app); registry != nil {
		return registry.Get(name)
	}
	return slog.Default()
}

func registryOf(app any) *Registry {
	switch value := app.(type) {
	case nil:
		return defaultRegistry()
	case *Registry:
		return value
	case runaprovider.Context:
		registry, _ := runaprovider.Invoke[*Registry](value)
		return registry
	case injectorProvider:
		registry, _ := do.Invoke[*Registry](value.Injector())
		return registry
	case logProvider:
		return value.Log()
	default:
		return nil
	}
}

func defaultRegistry() *Registry {
	injector := runaprovider.DefaultInjector()
	if injector == nil {
		return nil
	}
	registry, _ := do.Invoke[*Registry](injector)
	return registry
}
