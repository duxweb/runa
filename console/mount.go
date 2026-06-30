package console

import (
	"strings"

	authmiddleware "github.com/duxweb/runa/auth/middleware"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/route"
)

// Mount mounts console routes.
func Mount(target route.Target, app AppContext, configs ...Config) {
	if target == nil || app == nil {
		return
	}
	config := defaultConfig()
	if len(configs) > 0 {
		config = configs[0]
	}
	if config.Store == nil {
		config.Store = NewMemoryMonitorStore()
	}
	config = normalizeConfig(config)
	registry := New()
	if current, err := runaprovider.Invoke[*Registry](app); err == nil && current != nil {
		registry = current
	}
	provideMonitorStore(app, config.Store)
	provideSQLRecorder(app, config.Store)
	registry.RegisterBuiltinSummaries()
	registry.Register(config.Panels...)
	config.Panels = registry.Panels()
	group := target.RouteGroup()
	if len(config.Auth) > 0 {
		group.Use(authmiddleware.Use(config.Auth...))
	}
	group.Use(withRuntime(app, config, registry))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Redirect(firstPanelPath(ctx, config))
	}).SkipDoc()
	group.Get("/api/panels", func(ctx *route.Context) error {
		return ctx.JSON(panelInfo(config))
	}).SkipDoc()
	MountPanels(group, config.Panels...)
}

// MountPanels mounts extension panels under target.
func MountPanels(target route.Target, panels ...Panel) {
	if target == nil {
		return
	}
	group := target.RouteGroup()
	for _, panel := range panels {
		config := panelConfig(panel)
		if config.Name == "" {
			continue
		}
		panel.Routes(group.Group("/" + config.Name))
	}
}

func normalizeConfig(config Config) Config {
	if config.Title == "" {
		config.Title = defaultConfig().Title
	}
	if config.Interval <= 0 {
		config.Interval = defaultConfig().Interval
	}
	builtins := BuiltinPanels()
	panels := make([]Panel, 0, len(builtins)+len(config.Panels))
	for _, panel := range builtins {
		if !hasPanel(config.Panels, panelConfig(panel).Name) {
			panels = append(panels, panel)
		}
	}
	config.Panels = append(panels, config.Panels...)
	return config
}

func hasPanel(panels []Panel, name string) bool {
	for _, panel := range panels {
		if panelConfig(panel).Name == name {
			return true
		}
	}
	return false
}

func firstPanelPath(ctx *route.Context, config Config) string {
	items := panelInfo(config)
	if len(items) == 0 {
		return "."
	}
	if config.Mount != "" {
		return items[0].Path
	}
	path := strings.TrimRight(ctx.Request().URL.Path, "/")
	if path == "" {
		path = "/"
	}
	if path == "/" {
		return "/" + items[0].Name
	}
	return strings.TrimRight(path, "/") + "/" + items[0].Name
}
