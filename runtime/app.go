package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"sync"

	runacommand "github.com/duxweb/runa/command"
	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/host"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

// App is the Runa application runtime facade.
type App struct {
	basePath   string
	appPath    string
	configPath string
	dataPath   string
	publicPath string
	env        string
	timezone   string
	ctx        context.Context
	writer     io.Writer
	errWriter  io.Writer

	container       *do.RootScope
	bootedProviders []runaprovider.Provider
	bootedServices  []Service
	bootedModules   []Module
	pendingErrors   []error

	providers []runaprovider.Provider
	services  []Service
	modules   []Module
	commands  *runacommand.Registry
	hosts     *host.Manager

	frozen   bool
	freezing bool
	broken   bool
	stopped  bool
	cond     *sync.Cond
	mu       sync.Mutex
}

// New creates a Runa application.
func New(options ...Option) *App {
	app := &App{
		basePath:   defaultBasePath(),
		appPath:    "app",
		configPath: "config",
		dataPath:   "data",
		publicPath: "public",
		env:        defaultEnv(),
		ctx:        defaultContext(),
		writer:     os.Stdout,
		errWriter:  os.Stderr,
		container:  resetDefaultInjector(),
		commands:   runacommand.New(),
		hosts:      host.NewManager(),
	}
	for _, option := range options {
		if option != nil {
			option(app)
		}
	}
	app.cond = sync.NewCond(&app.mu)
	app.Command(serveCommand{}, inspectCommand{}, schemaCommand{})
	app.Install(
		config.Provider(app.BasePath(), app, app.ConfigPath(), app.Env(), "RUNA_", "APP_"),
	)
	return SetDefault(app)
}

func (app *App) recordError(err error) {
	if err == nil {
		return
	}
	app.mu.Lock()
	app.pendingErrors = append(app.pendingErrors, err)
	app.mu.Unlock()
}

func (app *App) pendingError() error {
	app.mu.Lock()
	defer app.mu.Unlock()
	return errors.Join(app.pendingErrors...)
}

// Env returns the application environment.
func (app *App) Env() string { return app.env }

// Writer returns the application output writer.
func (app *App) Writer() io.Writer { return app.writer }

// Install installs providers.
func (app *App) Install(providers ...runaprovider.Provider) *App {
	app.mu.Lock()
	app.providers = append(app.providers, providers...)
	app.mu.Unlock()
	return app
}

// Service registers services.
func (app *App) Service(services ...Service) *App {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.services = append(app.services, services...)
	return app
}

// Module registers modules.
func (app *App) Module(modules ...Module) *App {
	app.mu.Lock()
	defer app.mu.Unlock()
	for _, module := range modules {
		if module == nil {
			continue
		}
		app.modules = append(app.modules, module)
	}
	return app
}

// ModuleInfo returns module registration information.
func (app *App) ModuleInfo() []ModuleInfo {
	app.mu.Lock()
	modules := append([]Module(nil), app.modules...)
	frozen := app.frozen
	app.mu.Unlock()
	status := "registered"
	if frozen {
		status = "booted"
	}
	sorted, err := sortModules(modules)
	if err != nil {
		return moduleInfos(modules, status)
	}
	return moduleInfos(sorted, status)
}

// Command registers application commands.
func (app *App) Command(commands ...runacommand.Command) *App {
	app.commands.Register(commands...)
	return app
}

// Host registers application host units.
func (app *App) Host(units ...host.Unit) *App {
	app.mu.Lock()
	manager := app.hosts
	app.mu.Unlock()
	if err := manager.Register(units...); err != nil {
		app.recordError(err)
	}
	return app
}

// Hosts returns application host snapshots.
func (app *App) HostInfo() []host.Info {
	app.mu.Lock()
	manager := app.hosts
	app.mu.Unlock()
	return manager.Info()
}

// HostStatus returns a host unit status by name.
func (app *App) HostStatus(name string) host.Status {
	app.mu.Lock()
	manager := app.hosts
	app.mu.Unlock()
	return manager.Status(name)
}

// Run executes the CLI-first application entry.
func (app *App) Run(ctx context.Context) error {
	return app.Execute(ctx, os.Args[1:])
}

// Execute runs an application command. Empty args default to serve.
func (app *App) Execute(ctx context.Context, args []string) error {
	ctx = app.enterContext(ctx)
	if err := app.Freeze(ctx); err != nil {
		return err
	}

	if err := app.commands.Execute(ctx, app, args, runacommand.ExecuteConfig{
		Name:        "runa",
		Usage:       "Runa application",
		DefaultArgs: []string{"serve"},
		Writer:      app.writer,
		ErrWriter:   app.errWriter,
	}); err != nil {
		_ = app.Shutdown(DefaultContext())
		return err
	}
	return app.Shutdown(DefaultContext())
}

// Freeze compiles registrations and boots the app once.
func (app *App) Freeze(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	app.mu.Lock()
	for app.freezing {
		app.cond.Wait()
	}
	if app.broken {
		app.mu.Unlock()
		return fmt.Errorf("app freeze failed; create a new app instance")
	}
	if app.frozen {
		app.mu.Unlock()
		return nil
	}
	app.freezing = true
	app.mu.Unlock()

	err := app.freeze(ctx)
	if err == nil {
		app.mu.Lock()
		app.frozen = true
		app.mu.Unlock()
	} else {
		rollbackErr := app.rollback(ctx)
		if rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}
	app.mu.Lock()
	app.freezing = false
	app.cond.Broadcast()
	app.mu.Unlock()
	return err
}

func (app *App) freeze(ctx context.Context) error {
	app.mu.Lock()
	providers := append([]runaprovider.Provider(nil), app.providers...)
	services := append([]Service(nil), app.services...)
	modules := append([]Module(nil), app.modules...)
	app.mu.Unlock()
	sortProviders(providers)

	var err error
	modules, err = sortModules(modules)
	if err != nil {
		return err
	}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if err := provider.Init(ctx, app); err != nil {
			return fmt.Errorf("provider %s init: %w", provider.Name(), err)
		}
	}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if err := provider.Register(app); err != nil {
			return fmt.Errorf("provider %s register: %w", provider.Name(), err)
		}
	}
	if err := app.pendingError(); err != nil {
		return err
	}
	if err := app.applyTimezone(); err != nil {
		return err
	}

	app.mu.Lock()
	services = append([]Service(nil), app.services...)
	modules = append([]Module(nil), app.modules...)
	app.mu.Unlock()

	modules, err = sortModules(modules)
	if err != nil {
		return err
	}

	for _, service := range services {
		if err := service.Init(ctx, app); err != nil {
			return fmt.Errorf("service %s init: %w", service.Name(), err)
		}
	}
	for _, module := range modules {
		if err := module.Init(ctx, app); err != nil {
			return fmt.Errorf("module %s init: %w", module.Name(), err)
		}
	}
	for _, service := range services {
		if err := service.Register(ctx, app); err != nil {
			return fmt.Errorf("service %s register: %w", service.Name(), err)
		}
	}
	for _, module := range modules {
		if err := module.Register(ctx, app); err != nil {
			return fmt.Errorf("module %s register: %w", module.Name(), err)
		}
	}
	if err := app.pendingError(); err != nil {
		return err
	}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		resolver, ok := provider.(runaprovider.Resolver)
		if !ok {
			continue
		}
		if err := resolver.Resolve(ctx); err != nil {
			return fmt.Errorf("provider %s resolve: %w", provider.Name(), err)
		}
	}
	if err := app.pendingError(); err != nil {
		return err
	}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if err := provider.Boot(ctx, app); err != nil {
			return fmt.Errorf("provider %s boot: %w", provider.Name(), err)
		}
		app.recordBootedProvider(provider)
	}
	for _, service := range services {
		if err := service.Boot(ctx, app); err != nil {
			return fmt.Errorf("service %s boot: %w", service.Name(), err)
		}
		app.recordBootedService(service)
	}
	for _, module := range modules {
		if err := module.Boot(ctx, app); err != nil {
			return fmt.Errorf("module %s boot: %w", module.Name(), err)
		}
		app.recordBootedModule(module)
	}
	return nil
}

func sortProviders(providers []runaprovider.Provider) {
	sort.SliceStable(providers, func(i, j int) bool {
		return runaprovider.PriorityOf(providers[i]) < runaprovider.PriorityOf(providers[j])
	})
}

func (app *App) recordBootedProvider(provider runaprovider.Provider) {
	app.mu.Lock()
	app.bootedProviders = append(app.bootedProviders, provider)
	app.mu.Unlock()
}

func (app *App) recordBootedService(service Service) {
	app.mu.Lock()
	app.bootedServices = append(app.bootedServices, service)
	app.mu.Unlock()
}

func (app *App) recordBootedModule(module Module) {
	app.mu.Lock()
	app.bootedModules = append(app.bootedModules, module)
	app.mu.Unlock()
}

// Shutdown stops modules, services, and DI-managed resources.
func (app *App) Shutdown(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	app.mu.Lock()
	for app.freezing {
		app.cond.Wait()
	}
	if app.stopped {
		app.mu.Unlock()
		return nil
	}
	app.freezing = true
	app.mu.Unlock()

	err := app.shutdownApp(ctx)

	app.mu.Lock()
	app.stopped = true
	app.freezing = false
	app.cond.Broadcast()
	app.mu.Unlock()
	return err
}

func (app *App) rollback(ctx context.Context) error {
	err := app.shutdown(ctx, true)
	app.mu.Lock()
	app.bootedProviders = nil
	app.bootedServices = nil
	app.bootedModules = nil
	app.broken = true
	app.stopped = true
	app.mu.Unlock()
	return err
}

func (app *App) shutdownApp(ctx context.Context) error {
	return app.shutdown(ctx, false)
}

func (app *App) shutdown(ctx context.Context, rollback bool) error {
	app.mu.Lock()
	var modules []Module
	var services []Service
	var providers []runaprovider.Provider
	if rollback {
		modules = append([]Module(nil), app.bootedModules...)
		services = append([]Service(nil), app.bootedServices...)
		providers = append([]runaprovider.Provider(nil), app.bootedProviders...)
	} else {
		modules = append([]Module(nil), app.modules...)
		services = append([]Service(nil), app.services...)
		providers = append([]runaprovider.Provider(nil), app.bootedProviders...)
	}
	hosts := app.hosts
	app.mu.Unlock()

	if !rollback {
		if sorted, err := sortModules(modules); err == nil {
			modules = sorted
		}
	}

	var joined error
	if hosts != nil {
		if err := hosts.Stop(ctx); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	for i := len(modules) - 1; i >= 0; i-- {
		if err := modules[i].Shutdown(ctx, app); err != nil {
			joined = errors.Join(joined, fmt.Errorf("module %s shutdown: %w", modules[i].Name(), err))
		}
	}
	for i := len(services) - 1; i >= 0; i-- {
		if err := services[i].Shutdown(ctx, app); err != nil {
			joined = errors.Join(joined, fmt.Errorf("service %s shutdown: %w", services[i].Name(), err))
		}
	}
	for i := len(providers) - 1; i >= 0; i-- {
		if err := providers[i].Shutdown(ctx, app); err != nil {
			joined = errors.Join(joined, fmt.Errorf("provider %s shutdown: %w", providers[i].Name(), err))
		}
	}
	if app.container != nil {
		report := app.container.ShutdownWithContext(ctx)
		if report != nil && !report.Succeed {
			joined = errors.Join(joined, report)
		}
	}
	return joined
}

type serveCommand struct{}

func (serveCommand) Name() string    { return "serve" }
func (serveCommand) Summary() string { return "Start HTTP server" }
func (serveCommand) Run(ctx context.Context, command *runacommand.Context) error {
	app := command.App().(*App)
	if len(app.HostInfo()) == 0 {
		return errors.New("no host registered")
	}
	app.mu.Lock()
	hosts := app.hosts
	app.mu.Unlock()
	if err := hosts.Start(ctx); err != nil {
		return err
	}
	printServeHosts(app.writer, hosts.Info())
	<-ctx.Done()
	return nil
}

func printServeHosts(writer io.Writer, items []host.Info) {
	if writer == nil || len(items) == 0 {
		return
	}
	palette := servePalette{enabled: supportsServeColor(writer)}
	labelWidth := 0
	for _, item := range items {
		labelWidth = max(labelWidth, len(item.Name))
	}
	labelWidth = max(labelWidth, serveHostLabelMinWidth)
	fmt.Fprintf(writer, "%sHosts%s\n", palette.title(), palette.reset())
	for _, item := range items {
		status := string(item.Status)
		if item.Addr != "" {
			fmt.Fprintf(writer, "%s➜%s %s%-*s%s  %s%s%s   %s%s%s\n",
				palette.arrow(), palette.reset(),
				palette.label(), labelWidth, item.Name, palette.reset(),
				palette.status(item.Status), status, palette.reset(),
				palette.value(), displayHostAddr(item.Addr), palette.reset(),
			)
			continue
		}
		fmt.Fprintf(writer, "%s➜%s %s%-*s%s  %s%s%s\n",
			palette.arrow(), palette.reset(),
			palette.label(), labelWidth, item.Name, palette.reset(),
			palette.status(item.Status), status, palette.reset(),
		)
	}
}

func displayHostAddr(addr string) string {
	hostName, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if hostName == "" || hostName == "::" || hostName == "[::]" || hostName == "0.0.0.0" {
		hostName = "*"
	}
	return net.JoinHostPort(hostName, port)
}

type servePalette struct{ enabled bool }

const serveHostLabelMinWidth = 7

func (palette servePalette) reset() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[0m"
}

func (palette servePalette) arrow() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[36;1m"
}

func (palette servePalette) title() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[36m"
}

func (palette servePalette) label() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[32;1m"
}

func (palette servePalette) value() string {
	if !palette.enabled {
		return ""
	}
	return "\x1b[37m"
}

func (palette servePalette) status(status host.Status) string {
	if !palette.enabled {
		return ""
	}
	switch status {
	case host.Running:
		return "\x1b[32;1m"
	case host.Failed, host.Unhealthy:
		return "\x1b[31;1m"
	default:
		return "\x1b[37m"
	}
}

func supportsServeColor(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
}
