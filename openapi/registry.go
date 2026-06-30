package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	runacommand "github.com/duxweb/runa/command"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
)

// Registry stores OpenAPI document domains.
type Registry struct {
	docs map[string]Config
}

// New creates an OpenAPI registry.
func New() *Registry {
	return &Registry{docs: make(map[string]Config)}
}

// Register registers a document domain.
func (registry *Registry) Register(name string, options ...Option) Config {
	config := Config{Name: name, Title: name, Version: "1.0.0"}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return registry.RegisterConfig(config)
}

// RegisterConfig registers a document domain from an already merged config.
func (registry *Registry) RegisterConfig(config Config) Config {
	if config.Name == "" {
		config.Name = "default"
	}
	if config.Title == "" {
		config.Title = config.Name
	}
	if config.Version == "" {
		config.Version = "1.0.0"
	}
	if config.Viewer == nil {
		config.Viewer = ScalarViewer()
	}
	registry.docs[config.Name] = config
	return config
}

// Config returns a document domain config.
func (registry *Registry) Config(name string) (Config, bool) {
	config, ok := registry.docs[name]
	return config, ok
}

// List returns registered document domains.
func (registry *Registry) List() []Config {
	items := make([]Config, 0, len(registry.docs))
	for _, config := range registry.docs {
		items = append(items, config)
	}
	return items
}

// Document generates a document domain.
func (registry *Registry) Document(name string, routes []*route.Route) (Document, bool) {
	config, ok := registry.Config(name)
	if !ok {
		return Document{}, false
	}
	return Generate(config, routes), true
}

// JSONBytes generates JSON bytes for a document domain.
func (registry *Registry) JSONBytes(name string, routes []*route.Route) ([]byte, bool, error) {
	document, ok := registry.Document(name, routes)
	if !ok {
		return nil, false, nil
	}
	body, err := json.MarshalIndent(document, "", "  ")
	return body, true, err
}

// Mount registers OpenAPI JSON and UI routes.
func (registry *Registry) Mount(routes *route.Registry, name string, options ...Option) Config {
	config := registry.Register(name, options...)
	return registry.mountConfig(routes, config)
}

// MountConfig registers an OpenAPI config and mounts JSON/UI routes.
func (registry *Registry) MountConfig(routes *route.Registry, config Config) Config {
	config = registry.RegisterConfig(config)
	return registry.mountConfig(routes, config)
}

func (registry *Registry) mountConfig(routes *route.Registry, config Config) Config {
	if config.JSONPath != "" {
		routes.Get(config.JSONPath, func(ctx *route.Context) error {
			current, ok := registry.Config(config.Name)
			if !ok || current.JSONPath != ctx.Request().URL.Path {
				return ctx.Error(404, "OpenAPI document not found")
			}
			body, ok, err := registry.JSONBytes(config.Name, routes.Routes())
			if err != nil {
				return err
			}
			if !ok {
				return ctx.Error(404, "OpenAPI document not found")
			}
			return ctx.Blob(core.MIMEApplicationJSON, body)
		}).SkipDoc()
	}
	if config.UIPath != "" {
		routes.Get(config.UIPath, func(ctx *route.Context) error {
			current, ok := registry.Config(config.Name)
			if !ok || current.UIPath != ctx.Request().URL.Path {
				return ctx.Error(404, "OpenAPI UI not found")
			}
			if current.JSONPath == "" {
				return ctx.Error(404, "OpenAPI JSON path not configured")
			}
			return ctx.HTML(current.Viewer.HTML(current, current.JSONPath))
		}).SkipDoc()
	}
	return config
}

// ExportCommand creates an OpenAPI export command.
func ExportCommand(registry *Registry, routes *route.Registry) runacommand.Command {
	return exportCommand{registry: registry, routes: routes}
}

type exportCommand struct {
	registry *Registry
	routes   *route.Registry
}

func (exportCommand) Name() string    { return "openapi:export" }
func (exportCommand) Summary() string { return "Export OpenAPI document" }
func (exportCommand) Flags(flags *runacommand.FlagSet) {
	flags.String("doc", "", "OpenAPI document name")
	flags.String("out", "", "Output file path")
}
func (command exportCommand) Run(_ context.Context, cmd *runacommand.Context) error {
	configs := command.registry.List()
	if len(configs) == 0 {
		return fmt.Errorf("openapi document is not registered")
	}
	doc := cmd.Get[string]("doc")
	if doc == "" {
		doc = configs[0].Name
	}
	body, ok, err := command.registry.JSONBytes(doc, command.routes.Routes())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("openapi document %s is not registered", doc)
	}
	out := cmd.Get[string]("out")
	if out == "" {
		return cmd.Print(string(body))
	}
	return os.WriteFile(out, body, 0o644)
}
