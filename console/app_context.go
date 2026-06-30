package console

import (
	"github.com/duxweb/runa/host"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/route"
)

// AppContext is the narrow application context used by console resolvers.
type AppContext = runaprovider.Context

func appRoutes(app AppContext) []*route.Route {
	if app == nil {
		return nil
	}
	if registry, err := runaprovider.Invoke[*route.Registry](app); err == nil && registry != nil {
		return registry.Routes()
	}
	return nil
}

func appGroup(app AppContext, prefix string) *route.Group {
	if app == nil {
		return nil
	}
	if registry, err := runaprovider.Invoke[*route.Registry](app); err == nil && registry != nil {
		return registry.Group(prefix)
	}
	return nil
}

func appEnv(app AppContext) string {
	if app == nil {
		return ""
	}
	if runtime, ok := app.App().(interface{ Env() string }); ok {
		return runtime.Env()
	}
	return ""
}

func appHosts(app AppContext) []host.Info {
	if app == nil {
		return nil
	}
	if runtime, ok := app.App().(interface{ HostInfo() []host.Info }); ok {
		return runtime.HostInfo()
	}
	return nil
}
