package cluster

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	runaconfig "github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/id"
	runaprovider "github.com/duxweb/runa/provider"
)

// Registry stores cluster state for one app instance.
type Registry struct {
	options  options
	instance Instance
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.RWMutex
}

// Provider registers optional cluster heartbeat support.
func Provider(items ...Option) runaprovider.Provider {
	opts := defaultOptions()
	return clusterProvider{options: opts, items: append([]Option(nil), items...)}
}

type clusterProvider struct {
	runaprovider.Base
	options options
	items   []Option
}

func (provider clusterProvider) Name() string { return "cluster" }

func (item clusterProvider) Register(ctx runaprovider.Context) error {
	opts := item.options
	var cfg fileConfig
	if err := runaconfig.BindProvider(ctx, "cluster", "", &cfg); err != nil {
		return err
	}
	applyFileConfig(&opts, cfg)
	for _, option := range item.items {
		if option != nil {
			option(&opts)
		}
	}
	if opts.driver == nil {
		return fmt.Errorf("cluster driver is required")
	}
	runtime, err := NewRegistry(ctx, opts)
	if err != nil {
		return err
	}
	runaprovider.ProvideValueOnce(ctx, runtime)
	return ctx.RegisterService(runtime)
}

func applyFileConfig(opts *options, cfg fileConfig) {
	if cfg.Driver != "" {
		opts.driverName = cfg.Driver
		if opts.driver != nil && opts.driver.Name() != cfg.Driver {
			opts.driver = nil
		}
	}
	if cfg.Prefix != "" {
		opts.prefix = cfg.Prefix
	}
	if cfg.ID != "" {
		opts.id = cfg.ID
	}
	if cfg.Service != "" {
		opts.service = cfg.Service
	}
	if cfg.Env != "" {
		opts.env = cfg.Env
	}
	if cfg.Version != "" {
		opts.version = cfg.Version
	}
	if cfg.Addr != "" {
		opts.addr = cfg.Addr
	}
	if cfg.HeartbeatInterval > 0 {
		opts.heartbeatInterval = cfg.HeartbeatInterval
	}
	if cfg.TTL > 0 {
		opts.ttl = cfg.TTL
	}
	if cfg.Meta != nil {
		opts.meta = core.CloneMap(cfg.Meta)
	}
}

// NewRegistry creates a cluster runtime.
func NewRegistry(ctx runaprovider.Context, opts options) (*Registry, error) {
	if opts.driver == nil {
		return nil, fmt.Errorf("cluster driver is required")
	}
	hostname, _ := os.Hostname()
	instanceID := opts.id
	if instanceID == "" {
		instanceID = defaultInstanceID(hostname)
	}
	service := opts.service
	if service == "" {
		service = DefaultService
	}
	instance := Instance{
		ID:        instanceID,
		Service:   service,
		Env:       pick(opts.env, appEnv(ctx)),
		Version:   opts.version,
		Hostname:  hostname,
		PID:       os.Getpid(),
		Addr:      opts.addr,
		Status:    StatusStarting,
		StartedAt: core.Now(),
		TTL:       opts.ttl,
		Meta:      core.CloneMap(opts.meta),
	}
	return &Registry{options: opts, instance: instance}, nil
}

func (runtime *Registry) Name() string { return "cluster" }

func (runtime *Registry) Init(context.Context, runaprovider.Context) error { return nil }

func (runtime *Registry) Register(context.Context, runaprovider.Context) error { return nil }

func (runtime *Registry) Boot(ctx context.Context, app runaprovider.Context) error {
	ctx = normalizeContext(ctx)
	if err := runtime.resolveDriver(app); err != nil {
		return err
	}
	runtime.mu.Lock()
	if runtime.cancel != nil {
		runtime.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	runtime.cancel = cancel
	runtime.done = make(chan struct{})
	instance := runtime.instance
	instance.Status = StatusRunning
	instance.HeartbeatAt = core.Now()
	runtime.instance = instance
	runtime.mu.Unlock()

	if err := runtime.options.driver.Register(runCtx, instance); err != nil {
		cancel()
		runtime.mu.Lock()
		runtime.cancel = nil
		runtime.done = nil
		runtime.mu.Unlock()
		return err
	}
	go runtime.loop(runCtx, runtime.done)
	return nil
}

func (runtime *Registry) resolveDriver(app runaprovider.Context) error {
	_ = app
	if runtime.options.driver == nil {
		return fmt.Errorf("cluster driver %s is not registered", runtime.options.driverName)
	}
	if runtime.options.driverName != "" && runtime.options.driver.Name() != runtime.options.driverName {
		return fmt.Errorf("cluster driver %s is not registered", runtime.options.driverName)
	}
	return nil
}

func (runtime *Registry) Shutdown(ctx context.Context, app runaprovider.Context) error {
	_ = app
	ctx = normalizeContext(ctx)
	runtime.mu.Lock()
	cancel := runtime.cancel
	done := runtime.done
	runtime.cancel = nil
	runtime.done = nil
	instanceID := runtime.instance.ID
	runtime.instance.Status = StatusStopped
	runtime.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return runtime.options.driver.Unregister(context.Background(), instanceID)
}

func appEnv(ctx runaprovider.Context) string {
	if ctx == nil {
		return ""
	}
	if env, ok := ctx.App().(interface{ Env() string }); ok {
		return env.Env()
	}
	return ""
}

// Instance returns this runtime instance snapshot.
func (runtime *Registry) Instance() Instance {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return cloneInstance(runtime.instance)
}

// Instances returns active cluster instances.
func (runtime *Registry) Instances(ctx context.Context, service ...string) ([]Instance, error) {
	name := runtime.instance.Service
	if len(service) > 0 {
		name = service[0]
	}
	return runtime.options.driver.Instances(normalizeContext(ctx), name)
}

func (runtime *Registry) loop(ctx context.Context, done chan struct{}) {
	if done != nil {
		defer close(done)
	}
	ticker := time.NewTicker(runtime.options.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			runtime.mu.RLock()
			id := runtime.instance.ID
			status := runtime.instance.Status
			runtime.mu.RUnlock()
			_ = runtime.options.driver.Heartbeat(context.Background(), id, status)
		case <-ctx.Done():
			return
		}
	}
}

func defaultInstanceID(hostname string) string {
	value, err := id.Random(16)
	if err == nil {
		return value
	}
	return hostname
}

func pick(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
