package main

const moduleTemplate = `
package {{.Package}}

import (
	"context"

	"github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/route"
	{{.Package}}admin "{{.ModuleAdminImport}}"
)

// Module is the {{.Name}} business module entry.
type Module struct {
	provider.ModuleBase
}

// New creates the {{.Name}} module.
func New() Module { return Module{} }

func (Module) Name() string { return "{{.Name}}" }

func (Module) Register(ctx context.Context, app provider.Context) error {
	_ = ctx
	routes, err := provider.Invoke[*route.Registry](app)
	if err != nil {
		return err
	}
	{{.Package}}admin.Register(routes.Group("/{{.Name}}"))
	return nil
}
`

const moduleTestTemplate = `
package {{.Package}}

import "testing"

func TestModuleName(t *testing.T) {
	if New().Name() != "{{.Name}}" {
		t.Fatalf("unexpected module name")
	}
}
`

const adminRegisterTemplate = `
package admin

import "github.com/duxweb/runa/route"

// Register wires {{.Module}} admin routes into the module group.
func Register(group *route.Group) {
	group.Get("/", func(ctx *route.Context) error {
		return ctx.JSON(map[string]any{"module": "{{.Module}}"})
	}).Name("{{.Module}}.admin.index")
}
`

const adminRegisterTestTemplate = `
package admin

import (
	"testing"

	"github.com/duxweb/runa/route"
)

func TestRegister(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	Register(group)
	if len(registry.Routes()) == 0 {
		t.Fatalf("routes were not registered")
	}
}
`

const resourceTemplate = `
package {{.Package}}

import (
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/resource"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/validate"
)

type {{.Type}}ListInput struct{}

type {{.Type}}ShowInput struct {
	ID string ` + "`path:\"id\"`" + `
}

type {{.Type}}CreateInput struct {
	Name string ` + "`json:\"name\" form:\"name\"`" + `
}

func (input *{{.Type}}CreateInput) Validate(v *validate.Validator) {
	v.Field("name").Value(input.Name).Required("名称不能为空")
}

type {{.Type}}Output struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

// Register{{.Type}}Resource registers conventional resource routes.
func Register{{.Type}}Resource(group *route.Group) *resource.Resource {
	res := resource.New(group, "{{.RoutePath}}").Name("{{.Name}}").Summary("{{.Type}}").Tags("{{.Type}}")
	res.List[{{.Type}}ListInput, []{{.Type}}Output](func(ctx *route.Context, input *{{.Type}}ListInput) (*[]{{.Type}}Output, error) {
		items := []{{.Type}}Output{}
		return &items, nil
	})
	res.Show[{{.Type}}ShowInput, {{.Type}}Output](func(ctx *route.Context, input *{{.Type}}ShowInput) (*{{.Type}}Output, error) {
		return &{{.Type}}Output{ID: input.ID, Name: "{{.Name}}"}, nil
	})
	res.Create[{{.Type}}CreateInput, {{.Type}}Output](func(ctx *route.Context, input *{{.Type}}CreateInput) (*{{.Type}}Output, error) {
		return &{{.Type}}Output{ID: "new", Name: input.Name}, nil
	})
	res.Get[core.Empty, core.Map]("health", "/health", func(ctx *route.Context, input *core.Empty) (*core.Map, error) {
		out := core.Map{"ok": true}
		return &out, nil
	})
	return res
}
`

const resourceTestTemplate = `
package {{.Package}}

import (
	"testing"

	"github.com/duxweb/runa/route"
)

func TestRegister{{.Type}}Resource(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	res := Register{{.Type}}Resource(group)
	if res == nil || res.Route("list") == nil || res.Route("show") == nil || res.Route("create") == nil {
		t.Fatalf("resource routes were not registered")
	}
}
`

const crudTemplate = `
package {{.Package}}

import (
	"fmt"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/crud"
	"github.com/duxweb/runa/resource"
	"github.com/duxweb/runa/route"
)

type {{.Type}}Model struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type {{.Type}}Query struct {
	ID string
}

type {{.Type}}Output struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type {{.Var}}Store struct {
	items map[string]*{{.Type}}Model
}

func New{{.Type}}Store() *{{.Var}}Store {
	return &{{.Var}}Store{items: map[string]*{{.Type}}Model{}}
}

func (store *{{.Var}}Store) Query(ctx *crud.Context[{{.Type}}Model]) ({{.Type}}Query, error) {
	return {{.Type}}Query{ID: ctx.Param[string]("id")}, nil
}

func (store *{{.Var}}Store) List(ctx *crud.Context[{{.Type}}Model], query {{.Type}}Query) ([]*{{.Type}}Model, core.ListMeta, error) {
	items := make([]*{{.Type}}Model, 0, len(store.items))
	for _, item := range store.items {
		copy := *item
		items = append(items, &copy)
	}
	return items, core.PageMeta{Page: 1, PageSize: len(items), Total: len(items)}, nil
}

func (store *{{.Var}}Store) Show(ctx *crud.Context[{{.Type}}Model], query {{.Type}}Query) (*{{.Type}}Model, error) {
	item := store.items[query.ID]
	if item == nil {
		return nil, fmt.Errorf("{{.Name}} %s not found", query.ID)
	}
	copy := *item
	return &copy, nil
}

func (store *{{.Var}}Store) Create(ctx *crud.Context[{{.Type}}Model]) (*{{.Type}}Model, error) {
	if ctx.Model.ID == "" {
		ctx.Model.ID = fmt.Sprint(len(store.items) + 1)
	}
	copy := *ctx.Model
	store.items[copy.ID] = &copy
	return ctx.Model, nil
}

func (store *{{.Var}}Store) Edit(ctx *crud.Context[{{.Type}}Model], query {{.Type}}Query) (*{{.Type}}Model, error) {
	ctx.Model.ID = query.ID
	copy := *ctx.Model
	store.items[query.ID] = &copy
	return ctx.Model, nil
}

func (store *{{.Var}}Store) Store(ctx *crud.Context[{{.Type}}Model], query {{.Type}}Query, fields []string) (*{{.Type}}Model, error) {
	return store.Edit(ctx, query)
}

func (store *{{.Var}}Store) Delete(ctx *crud.Context[{{.Type}}Model], query {{.Type}}Query) error {
	delete(store.items, query.ID)
	return nil
}

func (store *{{.Var}}Store) Tx(ctx *crud.Context[{{.Type}}Model], fn func(ctx *crud.Context[{{.Type}}Model]) error) error {
	return fn(ctx)
}

// Register{{.Type}}CRUD registers CRUD routes. Replace New{{.Type}}Store with orostore when using database/oro.
func Register{{.Type}}CRUD(group *route.Group, store crud.Store[{{.Type}}Model, {{.Type}}Query]) *crud.Builder[{{.Type}}Model, {{.Type}}Query] {
	res := resource.New(group, "{{.RoutePath}}").Name("{{.Name}}").Summary("{{.Type}}").Tags("{{.Type}}")
	return crud.New[{{.Type}}Model, {{.Type}}Query](res, store).
		Transform[{{.Type}}Output](func(ctx *crud.Context[{{.Type}}Model], model *{{.Type}}Model) {{.Type}}Output {
			return {{.Type}}Output{ID: model.ID, Name: model.Name}
		})
}
`

const crudTestTemplate = `
package {{.Package}}

import (
	"testing"

	"github.com/duxweb/runa/route"
)

func TestRegister{{.Type}}CRUD(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	builder := Register{{.Type}}CRUD(group, New{{.Type}}Store())
	if builder == nil || builder.Route("list") == nil || builder.Route("show") == nil {
		t.Fatalf("crud routes were not registered")
	}
}
`

const providerTemplate = `
package {{.Package}}

import (
	"context"

	"github.com/duxweb/runa/provider"
)

type Registry struct{}

type Config struct {
	Enabled bool
}

type Option func(*Config)

func Enabled(value bool) Option {
	return func(config *Config) { config.Enabled = value }
}

type providerItem struct {
	provider.Base
	config Config
}

// Provider creates the {{.Name}} provider.
func Provider(options ...Option) provider.Provider {
	config := Config{Enabled: true}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return &providerItem{config: config}
}

func (providerItem) Name() string { return "{{.Name}}" }

func (item *providerItem) Init(ctx context.Context, app provider.Context) error {
	_ = ctx
	provider.ProvideValueOnce(app, &Registry{})
	return nil
}

func (item *providerItem) Register(ctx provider.Context) error {
	_ = ctx
	return nil
}

func (item *providerItem) Boot(ctx context.Context, app provider.Context) error {
	_, _ = ctx, app
	return nil
}
`

const providerTestTemplate = `
package {{.Package}}

import (
	"context"
	"testing"

	"github.com/duxweb/runa"
)

func TestProviderBoots(t *testing.T) {
	app := runa.New()
	app.Install(Provider())
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
}
`

const capabilityGoModTemplate = `module {{.GoModule}}/{{.Name}}

go 1.27rc1

require github.com/duxweb/runa v0.0.0
{{if .RootReplace}}
replace github.com/duxweb/runa => {{.RootReplace}}
{{end}}`

const capabilityTypesTemplate = `
package {{.Package}}

import "context"

// Driver is the {{.Name}} backend contract.
type Driver interface {
	Name() string
	Close(ctx context.Context) error
}
`

const capabilityOptionsTemplate = `
package {{.Package}}

type Options struct {
	Driver string
}

type ItemOption func(*Options)

// Use selects a registered driver by name.
func Use(name string) ItemOption {
	return func(options *Options) { options.Driver = name }
}

type Config struct {
	Drivers map[string]Driver
	Items   map[string][]ItemOption
}

type Option func(*Config)

// RegisterDriver registers a backend driver.
func RegisterDriver(name string, driver Driver) Option {
	return func(config *Config) {
		if name != "" && driver != nil {
			config.Drivers[name] = driver
		}
	}
}

// Register{{.Type}} registers a named {{.Name}} item.
func Register{{.Type}}(name string, options ...ItemOption) Option {
	return func(config *Config) {
		if name != "" {
			config.Items[name] = append([]ItemOption(nil), options...)
		}
	}
}
`

const capabilityRegistryTemplate = `
package {{.Package}}

import (
	"context"
	"fmt"
	"sync"
)

const DefaultDriver = "memory"

type entry struct {
	name    string
	options Options
}

// Registry stores {{.Name}} drivers and named entries.
type Registry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
	items   map[string]entry
}

// New creates a Registry.
func New() *Registry {
	return &Registry{drivers: make(map[string]Driver), items: make(map[string]entry)}
}

func (registry *Registry) RegisterDriver(name string, driver Driver) {
	if name == "" || driver == nil {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.drivers[name] = driver
}

func (registry *Registry) Register(name string, options ...ItemOption) {
	if name == "" {
		return
	}
	opts := Options{Driver: DefaultDriver}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	if opts.Driver == "" {
		opts.Driver = DefaultDriver
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.items[name] = entry{name: name, options: opts}
}

func (registry *Registry) Driver(name string) Driver {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.drivers[name]
}

func (registry *Registry) Freeze() error {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	for name, item := range registry.items {
		if registry.drivers[item.options.Driver] == nil {
			return fmt.Errorf("{{.Name}} %s driver %s is not registered", name, item.options.Driver)
		}
	}
	return nil
}

func (registry *Registry) Close(ctx context.Context) error {
	registry.mu.RLock()
	drivers := make([]Driver, 0, len(registry.drivers))
	for _, driver := range registry.drivers {
		drivers = append(drivers, driver)
	}
	registry.mu.RUnlock()
	for _, driver := range drivers {
		if err := driver.Close(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}
`

const capabilityProviderTemplate = `
package {{.Package}}

import (
	"context"

	"github.com/duxweb/runa/provider"
)

type providerItem struct {
	provider.Base
	config Config
}

// Provider creates the {{.Name}} provider.
func Provider(options ...Option) provider.Provider {
	config := Config{Drivers: make(map[string]Driver), Items: make(map[string][]ItemOption)}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return &providerItem{config: config}
}

func (providerItem) Name() string { return "{{.Name}}" }

func (item *providerItem) Init(ctx context.Context, app provider.Context) error {
	_ = ctx
	provider.ProvideValueOnce(app, New())
	return nil
}

func (item *providerItem) Register(ctx provider.Context) error {
	registry, err := provider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	for name, driver := range item.config.Drivers {
		registry.RegisterDriver(name, driver)
	}
	for name, options := range item.config.Items {
		registry.Register(name, options...)
	}
	return registry.Freeze()
}

func (item *providerItem) Shutdown(ctx context.Context, app provider.Context) error {
	registry, err := provider.Invoke[*Registry](app)
	if err != nil {
		return nil
	}
	return registry.Close(ctx)
}
`

const capabilityDefaultTemplate = `
package {{.Package}}

import "github.com/duxweb/runa/provider"

func Default() *Registry { return provider.MustInvokeDefault[*Registry]() }
`

const capabilityTestTemplate = `
package {{.Package}}

import (
	"context"
	"testing"
)

type testDriver struct{}

func (testDriver) Name() string { return "test" }
func (testDriver) Close(context.Context) error { return nil }

func TestRegistryRegisterDriver(t *testing.T) {
	registry := New()
	registry.RegisterDriver("test", testDriver{})
	registry.Register("default", Use("test"))
	if err := registry.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if registry.Driver("test") == nil {
		t.Fatalf("driver was not registered")
	}
}
`

const driverGoModTemplate = `module {{.GoModule}}/{{.Cap}}/{{.Name}}

go 1.27rc1

require {{.CapImport}} v0.0.0

replace {{.CapImport}} => ..
{{if .RootReplace}}
replace github.com/duxweb/runa => {{.RootReplace}}
{{end}}`

const driverOptionsTemplate = `
package {{.Package}}

type options struct{}

type Option func(*options)
`

const driverTemplate = `
package {{.Package}}

import (
	"context"

	{{.CapPackage}} "{{.CapImport}}"
)

// Driver creates a {{.Name}} driver.
func Driver(items ...Option) {{.CapPackage}}.Driver {
	opts := options{}
	for _, item := range items {
		if item != nil {
			item(&opts)
		}
	}
	return &driver{options: opts}
}

type driver struct {
	options options
}

func (driver *driver) Name() string { return "{{.Name}}" }

func (driver *driver) Close(ctx context.Context) error {
	_ = ctx
	return nil
}
`

const driverTestTemplate = `
package {{.Package}}

import "testing"

func TestDriver(t *testing.T) {
	if Driver() == nil {
		t.Fatalf("driver is nil")
	}
}
`
