package runtime

import (
	"context"
	"sort"

	runacommand "github.com/duxweb/runa/command"
	"github.com/duxweb/runa/host"
)

type inspectCommand struct{}

func (inspectCommand) Name() string    { return "inspect" }
func (inspectCommand) Summary() string { return "Inspect application graph" }
func (inspectCommand) Flags(flags *runacommand.FlagSet) {
	flags.Bool("json", true, "Output JSON")
}
func (inspectCommand) Run(_ context.Context, command *runacommand.Context) error {
	app := command.App().(*App)
	return command.JSON(app.Inspect())
}

type schemaCommand struct{}

func (schemaCommand) Name() string    { return "schema" }
func (schemaCommand) Summary() string { return "Export configuration JSON Schema" }
func (schemaCommand) Run(_ context.Context, command *runacommand.Context) error {
	return command.JSON(appConfigSchema())
}

type InspectInfo struct {
	Env      string       `json:"env"`
	BasePath string       `json:"base_path"`
	Modules  []ModuleInfo `json:"modules"`
	Hosts    []host.Info  `json:"hosts"`
	Commands []string     `json:"commands"`
	Services []string     `json:"services"`
}

func (app *App) Inspect() InspectInfo {
	services := []string{}
	for _, item := range app.container.ListProvidedServices() {
		services = append(services, item.Service)
	}
	sort.Strings(services)
	return InspectInfo{
		Env:      app.Env(),
		BasePath: app.BasePath(),
		Modules:  app.ModuleInfo(),
		Hosts:    app.HostInfo(),
		Commands: app.commandNames(),
		Services: services,
	}
}

func (app *App) commandNames() []string {
	items := app.commands.Names()
	sort.Strings(items)
	return items
}

func appConfigSchema() map[string]any {
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"title":   "Runa configuration",
		"type":    "object",
		"properties": map[string]any{
			"app": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"env":      map[string]any{"type": "string"},
					"timezone": map[string]any{"type": "string"},
				},
			},
			"queue": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"queues":  map[string]any{"type": "object"},
					"workers": map[string]any{"type": "object"},
				},
			},
		},
		"additionalProperties": true,
	}
}
