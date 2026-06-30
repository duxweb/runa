package console

import (
	"context"
	"strings"
	"sync"
	"time"

	runaconfig "github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/database"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
)

// Provider registers and optionally mounts console.
func Provider(options ...Option) runaprovider.Provider {
	config := defaultConfig()
	return consoleProvider{config: config, options: append([]Option(nil), options...)}
}

type consoleProvider struct {
	runaprovider.Base
	config  Config
	options []Option
}

func (provider consoleProvider) Name() string { return "console" }

func (item consoleProvider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return New(), nil })
	return nil
}

func (item consoleProvider) Register(ctx runaprovider.Context) error {
	app := AppContext(ctx)
	config := item.config
	if err := runaconfig.BindProvider(ctx, "console", "", &config); err != nil {
		return err
	}
	for _, option := range item.options {
		if option != nil {
			option(&config)
		}
	}
	if config.Store == nil {
		config.Store = NewMemoryMonitorStore()
	}
	runaprovider.ProvideValueOnce[MonitorStore](ctx, config.Store)
	runaprovider.ProvideValueOnce[database.SQLRecorder](ctx, config.Store)
	Register(app, BuiltinPanels()...)
	Register(app, config.Panels...)
	RegisterBuiltinSummaries(app)
	return ctx.RegisterService(&service{config: config})
}

func provideMonitorStore(app AppContext, store MonitorStore) {
	if app == nil || store == nil {
		return
	}
	runaprovider.ProvideValueOnceTo[MonitorStore](app.Injector(), store)
}

func provideSQLRecorder(app AppContext, store MonitorStore) {
	if app == nil || store == nil {
		return
	}
	runaprovider.ProvideValueOnceTo[database.SQLRecorder](app.Injector(), store)
}

type service struct {
	runaprovider.ServiceBase
	config Config
	stop   chan struct{}
	once   sync.Once
}

func (service *service) Name() string { return "console" }
func (service *service) Register(_ context.Context, app AppContext) error {
	installExecutionMonitors(app, MonitorStoreOf(app))
	if service.config.Mount == "" {
		return nil
	}
	group := appGroup(app, service.config.Mount)
	if group == nil {
		return nil
	}
	Mount(group, app, service.config)
	return nil
}

func (service *service) Boot(_ context.Context, app AppContext) error {
	store := MonitorStoreOf(app)
	if !service.config.CollectHTTP {
		return service.startSampler(app, store)
	}
	monitor := monitorMiddleware(store, service.config.SlowThreshold)
	mount := strings.TrimRight(service.config.Mount, "/")
	for _, item := range appRoutes(app) {
		if item == nil || (mount != "" && (item.Path == mount || strings.HasPrefix(item.Path, mount+"/"))) {
			continue
		}
		item.Use(monitor)
	}
	return service.startSampler(app, store)
}

func (service *service) Shutdown(context.Context, AppContext) error {
	service.once.Do(func() {
		if service.stop != nil {
			close(service.stop)
		}
	})
	return nil
}

func (service *service) startSampler(app AppContext, store MonitorStore) error {
	interval := service.config.SampleInterval
	if interval <= 0 {
		interval = defaultConfig().SampleInterval
	}
	service.stop = make(chan struct{})
	sampleQueue(app, store)
	sampleWS(app, store)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sampleQueue(app, store)
				sampleWS(app, store)
			case <-service.stop:
				return
			}
		}
	}()
	return nil
}

func sampleQueue(app AppContext, store MonitorStore) {
	if app == nil || store == nil {
		return
	}
	sample := QueueSample{Time: core.Now()}
	for _, item := range queueInfos(context.Background(), app) {
		sample.Pending += item.Pending
		sample.Reserved += item.Reserved
		sample.Delayed += item.Delayed
		sample.Failed += item.Failed
	}
	for _, item := range workerInfos(context.Background(), app) {
		sample.Processed += item.Processed
		sample.Succeeded += item.Succeeded
		sample.Failed += item.Failed
		sample.Retried += item.Retried
		sample.Workers++
		sample.Instances += item.Instances
	}
	store.RecordQueue(sample)
}
