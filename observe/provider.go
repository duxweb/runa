package observe

import (
	"context"
	"expvar"
	"net/http"
	"net/http/pprof"
	"runtime"
	"time"

	"github.com/duxweb/runa/cache"
	runaconfig "github.com/duxweb/runa/config"
	"github.com/duxweb/runa/database"
	"github.com/duxweb/runa/host"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/schedule"
	"github.com/duxweb/runa/storage"
)

// Exporter renders metrics for an HTTP request.
type Exporter interface {
	Serve(ctx *route.Context) error
}

// ExporterFunc adapts a function to Exporter.
type ExporterFunc func(ctx *route.Context) error

// Serve renders metrics.
func (fn ExporterFunc) Serve(ctx *route.Context) error { return fn(ctx) }

// Installer installs trace propagation or instrumentation hooks.
type Installer interface {
	Install(ctx runaprovider.Context) error
}

// InstallerFunc adapts a function to Installer.
type InstallerFunc func(ctx runaprovider.Context) error

// Install installs trace hooks.
func (fn InstallerFunc) Install(ctx runaprovider.Context) error {
	return fn(ctx)
}

type state struct {
	app     runaprovider.Context
	config  Config
	health  *Registry
	ready   *Registry
	metrics Exporter
	traces  []Installer
	started time.Time
}

// Provider returns a Runa provider that registers observe endpoints.
func Provider(config Config, options ...Option) runaprovider.Provider {
	current := &state{
		config:  defaultConfig(config),
		health:  New(),
		ready:   New(),
		started: time.Now(),
	}
	current.health.Add("self", Self())
	return provider{state: current, config: config, options: append([]Option(nil), options...)}
}

type provider struct {
	runaprovider.Base
	state   *state
	config  Config
	options []Option
}

func (provider provider) Name() string { return "observe" }

func (item provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideValueOnce(ctx, item.state.health)
	return nil
}

func (item provider) Register(ctx runaprovider.Context) error {
	item.state.app = ctx
	config := item.config
	if err := runaconfig.BindProvider(ctx, "observe", "", &config); err != nil {
		return err
	}
	item.state.config = defaultConfig(config)
	for _, option := range item.options {
		if option != nil {
			option(item.state)
		}
	}
	for _, installer := range item.state.traces {
		if installer != nil {
			if err := installer.Install(ctx); err != nil {
				return err
			}
		}
	}
	mount := item.state.config.Mount
	if mount == "" {
		mount = "/"
	}
	routes, err := runaprovider.Invoke[*route.Registry](ctx)
	if err != nil {
		return err
	}
	Mount(routes.Group(mount), item.state)
	return nil
}

// Mount mounts observe endpoints into a route target.
func Mount(target route.Target, runtimes ...*state) {
	if target == nil {
		return
	}
	current := firstRuntime(runtimes...)
	group := target.RouteGroup()
	group.Get("/health", func(ctx *route.Context) error {
		return writeReport(ctx, current.health.Run(ctx.Context(), current.config.Timeout))
	}).Meta("observe", true).SkipDoc()
	group.Get("/ready", func(ctx *route.Context) error {
		return writeReport(ctx, current.ready.Run(ctx.Context(), current.config.Timeout))
	}).Meta("observe", true).SkipDoc()
	if current.metrics != nil {
		group.Get("/metrics", current.metrics.Serve).Meta("observe", true).SkipDoc()
	}
	if current.config.Debug {
		group.Get("/debug/monitor", func(ctx *route.Context) error {
			return ctx.JSON(current.Monitor(ctx.Context()))
		}).Meta("observe", true).SkipDoc()
		mountDebug(group)
	}
}

// Monitor returns current application runtime status.
func (state *state) Monitor(ctx context.Context) map[string]any {
	var mem runtimeMemStats
	mem.Read()
	data := map[string]any{
		"service": state.config.Service,
		"env":     state.config.Env,
		"version": state.config.Version,
		"uptime":  time.Since(state.started).String(),
		"runtime": map[string]any{
			"goos":       goos(),
			"goarch":     goarch(),
			"gomaxprocs": gomaxprocs(),
		},
		"memory":    mem.Map(),
		"goroutine": numGoroutine(),
	}
	if state.app == nil {
		return data
	}
	if routes, err := runaprovider.Invoke[*route.Registry](state.app); err == nil && routes != nil {
		data["route"] = map[string]any{"count": len(routes.Routes())}
	}
	if hosts, ok := state.app.App().(interface{ HostInfo() []host.Info }); ok {
		data["host"] = hosts.HostInfo()
	}
	if registry, err := runaprovider.Invoke[*database.Registry](state.app); err == nil && registry != nil {
		data["database"] = registry.Info()
	}
	if registry, err := runaprovider.Invoke[*cache.Registry](state.app); err == nil && registry != nil {
		data["cache"] = registry.Info()
	}
	if registry, err := runaprovider.Invoke[*queue.Registry](state.app); err == nil && registry != nil {
		data["queue"] = registry.QueueInfo(ctx)
		data["worker"] = registry.WorkerInfo(ctx)
	}
	if registry, err := runaprovider.Invoke[*schedule.Registry](state.app); err == nil && registry != nil {
		data["schedule"] = registry.Info()
	}
	if registry, err := runaprovider.Invoke[*storage.Registry](state.app); err == nil && registry != nil {
		data["storage"] = registry.Info()
	}
	return data
}

func writeReport(ctx *route.Context, report Report) error {
	if report.Status == Fail {
		return ctx.Status(http.StatusServiceUnavailable).JSON(report)
	}
	return ctx.JSON(report)
}

func firstRuntime(values ...*state) *state {
	if len(values) > 0 && values[0] != nil {
		return values[0]
	}
	return &state{config: defaultConfig(Config{}), health: New(), ready: New(), started: time.Now()}
}

func mountDebug(group *route.Group) {
	group.Get("/debug/pprof", stdHandler(pprof.Index)).Meta("observe", true).SkipDoc()
	group.Get("/debug/pprof/", stdHandler(pprof.Index)).Meta("observe", true).SkipDoc()
	group.Get("/debug/pprof/cmdline", stdHandler(pprof.Cmdline)).Meta("observe", true).SkipDoc()
	group.Get("/debug/pprof/profile", stdHandler(pprof.Profile)).Meta("observe", true).SkipDoc()
	group.Get("/debug/pprof/symbol", stdHandler(pprof.Symbol)).Meta("observe", true).SkipDoc()
	group.Get("/debug/pprof/trace", stdHandler(pprof.Trace)).Meta("observe", true).SkipDoc()
	group.Get("/debug/vars", stdHandler(expvar.Handler().ServeHTTP)).Meta("observe", true).SkipDoc()
}

func stdHandler(handler func(http.ResponseWriter, *http.Request)) route.Handler {
	return func(ctx *route.Context) error {
		handler(ctx.Response(), ctx.Request())
		return nil
	}
}

type runtimeMemStats struct {
	stats runtime.MemStats
}

func (stats *runtimeMemStats) Read() { runtime.ReadMemStats(&stats.stats) }
func (stats runtimeMemStats) Map() map[string]uint64 {
	return map[string]uint64{
		"alloc":       stats.stats.Alloc,
		"total_alloc": stats.stats.TotalAlloc,
		"sys":         stats.stats.Sys,
		"heap_alloc":  stats.stats.HeapAlloc,
		"heap_sys":    stats.stats.HeapSys,
	}
}

func goos() string      { return runtime.GOOS }
func goarch() string    { return runtime.GOARCH }
func gomaxprocs() int   { return runtime.GOMAXPROCS(0) }
func numGoroutine() int { return runtime.NumGoroutine() }
