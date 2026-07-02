package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	iregistry "github.com/duxweb/runa/kernel/registry"
)

const (
	DefaultDriver       = "memory"
	DefaultQueue        = "default"
	DefaultWorker       = "default"
	DefaultRetry        = 0
	DefaultRetryDelay   = time.Second
	DefaultPollInterval = 100 * time.Millisecond
	DefaultLease        = 30 * time.Second
	DefaultRetention    = 7 * 24 * time.Hour
	DefaultConcurrency  = 10
	DefaultStopTimeout  = 30 * time.Second
)

// Registry stores queue drivers, queues, workers, and handlers.
type Registry struct {
	mu              sync.RWMutex
	drivers         map[string]Driver
	queues          map[string]queueEntry
	workers         map[string]workerEntry
	jobs            map[string]jobEntry
	stats           map[string]*workerStats
	middlewares     []Middleware
	pushMiddlewares []PushMiddleware
	ids             atomic.Uint64
}

// New creates a registry.
func New() *Registry {
	registry := &Registry{
		drivers: make(map[string]Driver),
		queues:  make(map[string]queueEntry),
		workers: make(map[string]workerEntry),
		jobs:    make(map[string]jobEntry),
		stats:   make(map[string]*workerStats),
	}
	registry.RegisterDriver(DefaultDriver, MemoryDriver())
	registry.Worker(DefaultWorker)
	registry.Queue(DefaultQueue, Workers(DefaultWorker))
	return registry
}

// RegisterDriver registers a queue driver.
func (registry *Registry) RegisterDriver(name string, driver Driver) {
	if name == "" || driver == nil {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.drivers[name] = driver
}

// Queue registers a queue.
func (registry *Registry) Queue(name string, options ...QueueOption) {
	if name == "" {
		return
	}
	opts := applyQueueOptions(options...)
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if opts.Workers == nil {
		if entry, ok := registry.queues[name]; ok {
			opts.Workers = append([]string(nil), entry.options.Workers...)
		}
	}
	registry.queues[name] = queueEntry{name: name, options: opts, code: append([]QueueOption(nil), options...)}
}

// Worker registers a worker.
func (registry *Registry) Worker(name string, options ...WorkerOption) {
	if name == "" {
		return
	}
	opts := applyWorkerOptions(options...)
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.workers[name] = workerEntry{name: name, options: opts, code: append([]WorkerOption(nil), options...)}
	if registry.stats[name] == nil {
		registry.stats[name] = &workerStats{}
	}
}

// Config applies file/env config to already registered queues and workers.
func (registry *Registry) Config(store *config.Store) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for name, entry := range registry.queues {
		options := append(configQueueOptions(store, name), entry.code...)
		opts := applyQueueOptions(options...)
		if opts.Workers == nil {
			opts.Workers = append([]string(nil), entry.options.Workers...)
		}
		entry.options = opts
		registry.queues[name] = entry
	}
	for name, entry := range registry.workers {
		options := append(configWorkerOptions(store, name), entry.code...)
		entry.options = applyWorkerOptions(options...)
		registry.workers[name] = entry
		if registry.stats[name] == nil {
			registry.stats[name] = &workerStats{}
		}
	}
}

func applyQueueOptions(options ...QueueOption) QueueOptions {
	opts := QueueOptions{
		Driver:     DefaultDriver,
		Retry:      DefaultRetry,
		RetryDelay: DefaultRetryDelay,
		Retention:  DefaultRetention,
		Meta:       make(map[string]any),
	}
	for _, option := range options {
		if option != nil {
			option.ApplyQueue(&opts)
		}
	}
	if opts.Driver == "" {
		opts.Driver = DefaultDriver
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = DefaultRetryDelay
	}
	if !opts.retentionSet {
		opts.Retention = DefaultRetention
	}
	return opts
}

func applyWorkerOptions(options ...WorkerOption) WorkerOptions {
	opts := WorkerOptions{
		Concurrency:  DefaultConcurrency,
		PollInterval: DefaultPollInterval,
		Lease:        DefaultLease,
		StopTimeout:  DefaultStopTimeout,
		Meta:         make(map[string]any),
	}
	for _, option := range options {
		if option != nil {
			option.ApplyWorker(&opts)
		}
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = DefaultConcurrency
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = DefaultPollInterval
	}
	if opts.Lease <= 0 {
		opts.Lease = DefaultLease
	}
	if opts.StopTimeout <= 0 {
		opts.StopTimeout = DefaultStopTimeout
	}
	return opts
}

// Use adds worker job execution middleware without replacing existing worker options.
func (registry *Registry) Use(worker string, middlewares ...Middleware) {
	if worker == All {
		registry.mu.Lock()
		for _, middleware := range middlewares {
			if middleware != nil {
				registry.middlewares = append(registry.middlewares, middleware)
			}
		}
		registry.mu.Unlock()
		return
	}
	if worker == "" {
		worker = DefaultWorker
	}
	registry.mu.Lock()
	entry, ok := registry.workers[worker]
	if ok {
		for _, middleware := range middlewares {
			if middleware != nil {
				entry.options.Middlewares = append(entry.options.Middlewares, middleware)
			}
		}
		registry.workers[worker] = entry
	}
	registry.mu.Unlock()
}

// UsePush adds middleware to every queue push.
func (registry *Registry) UsePush(middlewares ...PushMiddleware) {
	registry.mu.Lock()
	for _, middleware := range middlewares {
		if middleware != nil {
			registry.pushMiddlewares = append(registry.pushMiddlewares, middleware)
		}
	}
	registry.mu.Unlock()
}

// Job registers a typed job handler.
func (registry *Registry) Job[T any](name string, handler Handler[T], options ...JobOption) {
	if name == "" || handler == nil {
		return
	}
	opts := JobOptions{
		Retry: DefaultRetry,
		Meta:  make(map[string]any),
	}
	for _, option := range options {
		if option != nil {
			option.ApplyJob(&opts)
		}
	}
	payloadType := core.TypeOf[T]()
	entry := jobEntry{
		name:        name,
		payloadType: payloadType,
		payloadName: core.TypeName(payloadType),
		options:     opts,
		call: func(ctx context.Context, message *JobMessage) error {
			var payload T
			if len(message.Payload) > 0 {
				if err := json.Unmarshal(message.Payload, &payload); err != nil {
					return err
				}
			}
			return handler(ctx, &Job[T]{
				ID:         message.ID,
				Queue:      message.Queue,
				Name:       message.Name,
				Payload:    payload,
				Meta:       core.CloneMap(message.Meta),
				Attempt:    message.Attempt,
				MaxAttempt: message.MaxAttempt,
				CreatedAt:  message.CreatedAt,
				RunAt:      message.RunAt,
				Timeout:    message.Timeout,
			})
		},
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.jobs[name] = entry
}

// Push serializes and pushes a typed job into a queue.
func (registry *Registry) Push[T any](ctx context.Context, queueName string, name string, payload T, options ...PushOption) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	opts := PushOptions{Meta: make(map[string]any)}
	for _, option := range options {
		if option != nil {
			option.ApplyPush(&opts)
		}
	}
	return registry.PushMessage(ctx, queueName, name, body, opts)
}

// PushMessage pushes a pre-serialized payload into a queue.
func (registry *Registry) PushMessage(ctx context.Context, queueName string, name string, payload []byte, options PushOptions) (string, error) {
	ctx = core.NormalizeContext(ctx)
	if queueName == "" {
		queueName = DefaultQueue
	}
	queue, driver, job, err := registry.pushRefs(queueName, name)
	if err != nil {
		return "", err
	}
	now := core.Now()
	timeout := options.Timeout
	if timeout == 0 {
		timeout = job.options.Timeout
	}
	if timeout == 0 {
		timeout = queue.options.Timeout
	}
	maxAttempt := options.Retry
	if maxAttempt == 0 {
		maxAttempt = job.options.Retry
	}
	if maxAttempt == 0 {
		maxAttempt = queue.options.Retry
	}
	retryDelay := options.RetryDelay
	if retryDelay == 0 {
		retryDelay = job.options.RetryDelay
	}
	if retryDelay == 0 {
		retryDelay = queue.options.RetryDelay
	}
	id, err := registry.nextID()
	if err != nil {
		return "", err
	}
	message := &JobMessage{
		ID:             id,
		Queue:          queue.name,
		Name:           name,
		Payload:        append([]byte(nil), payload...),
		Meta:           mergeMap(queue.options.Meta, job.options.Meta, options.Meta),
		MaxAttempt:     maxAttempt,
		CreatedAt:      now,
		RunAt:          now.Add(options.Delay),
		Timeout:        timeout,
		RetryDelay:     retryDelay,
		Unique:         options.Unique,
		UniqueStrategy: normalizeUniqueStrategy(options.Unique, options.UniqueStrategy),
		UniqueTTL:      options.UniqueTTL,
		UpdatedAt:      now,
		Attempt:        0,
		LastError:      "",
	}
	call := PushHandlerFunc(func(ctx context.Context, queue string, job *JobMessage) (string, error) {
		return driver.Push(ctx, queue, job)
	})
	middlewares := registry.pushMiddlewareSnapshot()
	for i := len(middlewares) - 1; i >= 0; i-- {
		call = middlewares[i](call)
	}
	return call(ctx, queue.name, message)
}

func normalizeUniqueStrategy(unique string, strategy string) string {
	if unique == "" {
		return ""
	}
	if strategy == string(UniqueStrategyUntilStart) {
		return string(UniqueStrategyUntilStart)
	}
	return string(UniqueStrategyUntilDone)
}

// Freeze validates all registrations and marks the registry as readonly.
func (registry *Registry) Freeze() error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for name, entry := range registry.queues {
		if registry.drivers[entry.options.Driver] == nil {
			return fmt.Errorf("queue %s driver %s is not registered", name, entry.options.Driver)
		}
		for _, worker := range entry.options.Workers {
			if _, ok := registry.workers[worker]; !ok {
				return fmt.Errorf("queue %s worker %s is not registered", name, worker)
			}
		}
	}
	for name := range registry.workers {
		if len(registry.workerQueueNamesLocked(name)) == 0 {
			return fmt.Errorf("worker %s has no queues", name)
		}
	}
	for name, entry := range registry.jobs {
		if entry.payloadType == nil {
			return fmt.Errorf("job %s payload type is required", name)
		}
	}
	return nil
}

// QueueInfo returns queue snapshots with driver counts.
func (registry *Registry) QueueInfo(ctx context.Context) []QueueInfo {
	ctx = core.NormalizeContext(ctx)
	entries := registry.queueEntries()
	items := make([]QueueInfo, 0, len(entries))
	for _, entry := range entries {
		driver := registry.driver(entry.options.Driver)
		item := QueueInfo{
			Name:    entry.name,
			Driver:  entry.options.Driver,
			Workers: append([]string(nil), entry.options.Workers...),
			Meta:    core.CloneMap(entry.options.Meta),
		}
		if driver != nil {
			item.Pending, _ = driver.Count(ctx, entry.name, StatePending)
			item.Delayed, _ = driver.Count(ctx, entry.name, StateDelayed)
			item.Reserved, _ = driver.Count(ctx, entry.name, StateReserved)
			item.Failed, _ = driver.Count(ctx, entry.name, StateFailed)
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// WorkerInfo returns worker snapshots.
func (registry *Registry) WorkerInfo(ctx context.Context) []WorkerInfo {
	ctx = core.NormalizeContext(ctx)
	entries := registry.workerEntries()
	items := make([]WorkerInfo, 0, len(entries))
	for _, entry := range entries {
		stats := registry.workerStats(entry.name)
		instances := registry.instances(ctx, entry)
		items = append(items, WorkerInfo{
			Name:        entry.name,
			Queues:      registry.workerQueueNames(entry.name),
			Concurrency: entry.options.Concurrency,
			Instances:   len(instances),
			Status:      workerStatus(instances),
			Processed:   stats.processed.Load(),
			Succeeded:   stats.succeeded.Load(),
			Failed:      stats.failed.Load(),
			Retried:     stats.retried.Load(),
			Meta:        core.CloneMap(entry.options.Meta),
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// JobInfo returns registered job handler snapshots.
func (registry *Registry) JobInfo() []JobInfo {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]JobInfo, 0, len(registry.jobs))
	for name, item := range registry.jobs {
		items = append(items, JobInfo{Name: name, Payload: item.payloadName, Source: "app", Meta: core.CloneMap(item.options.Meta)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// Close closes all registered drivers.
func (registry *Registry) Close(ctx context.Context) error {
	registry.mu.RLock()
	drivers := make(map[string]Driver, len(registry.drivers))
	for name, driver := range registry.drivers {
		drivers[name] = driver
	}
	registry.mu.RUnlock()
	return iregistry.CloseAll(ctx, drivers, "queue driver")
}

// Shutdown closes all queue drivers when managed by DI.
func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}

func (registry *Registry) pushRefs(queueName string, jobName string) (queueEntry, Driver, jobEntry, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	queue, ok := registry.queues[queueName]
	if !ok {
		return queueEntry{}, nil, jobEntry{}, fmt.Errorf("queue %s is not registered", queueName)
	}
	driver := registry.drivers[queue.options.Driver]
	if driver == nil {
		return queueEntry{}, nil, jobEntry{}, fmt.Errorf("queue driver %s is not registered", queue.options.Driver)
	}
	job, ok := registry.jobs[jobName]
	if !ok {
		return queueEntry{}, nil, jobEntry{}, fmt.Errorf("job %s is not registered", jobName)
	}
	return queue, driver, job, nil
}

func (registry *Registry) worker(name string) (workerEntry, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entry, ok := registry.workers[name]
	if !ok {
		return workerEntry{}, fmt.Errorf("worker %s is not registered", name)
	}
	return entry, nil
}

func (registry *Registry) queue(name string) (queueEntry, Driver, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entry, ok := registry.queues[name]
	if !ok {
		return queueEntry{}, nil, fmt.Errorf("queue %s is not registered", name)
	}
	driver := registry.drivers[entry.options.Driver]
	if driver == nil {
		return queueEntry{}, nil, fmt.Errorf("queue driver %s is not registered", entry.options.Driver)
	}
	return entry, driver, nil
}

func (registry *Registry) driver(name string) Driver {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.drivers[name]
}

func (registry *Registry) job(name string) (jobEntry, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entry, ok := registry.jobs[name]
	if !ok {
		return jobEntry{}, fmt.Errorf("job %s is not registered", name)
	}
	return entry, nil
}

func (registry *Registry) queueEntries() []queueEntry {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]queueEntry, 0, len(registry.queues))
	for _, entry := range registry.queues {
		items = append(items, entry)
	}
	return items
}

func (registry *Registry) workerEntries() []workerEntry {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]workerEntry, 0, len(registry.workers))
	for _, entry := range registry.workers {
		items = append(items, entry)
	}
	return items
}

func (registry *Registry) workerStats(name string) *workerStats {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	stats := registry.stats[name]
	if stats == nil {
		stats = &workerStats{}
		registry.stats[name] = stats
	}
	return stats
}

func (registry *Registry) middlewareSnapshot() []Middleware {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return append([]Middleware(nil), registry.middlewares...)
}

func (registry *Registry) pushMiddlewareSnapshot() []PushMiddleware {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return append([]PushMiddleware(nil), registry.pushMiddlewares...)
}

func (registry *Registry) instances(ctx context.Context, entry workerEntry) []WorkerInstance {
	seen := map[string]struct{}{}
	var items []WorkerInstance
	for _, queueName := range registry.workerQueueNames(entry.name) {
		_, driver, err := registry.queue(queueName)
		if err != nil {
			continue
		}
		state, ok := driver.(WorkerState)
		if !ok {
			continue
		}
		instances, err := state.Instances(ctx, entry.name)
		if err != nil {
			continue
		}
		for _, instance := range instances {
			if _, ok := seen[instance.ID]; ok {
				continue
			}
			seen[instance.ID] = struct{}{}
			items = append(items, instance)
		}
	}
	return items
}

func (registry *Registry) workerQueueNames(name string) []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.workerQueueNamesLocked(name)
}

func (registry *Registry) workerQueueNamesLocked(name string) []string {
	var names []string
	seen := map[string]struct{}{}
	for queueName, entry := range registry.queues {
		for _, worker := range entry.options.Workers {
			if worker != name {
				continue
			}
			if _, ok := seen[queueName]; ok {
				continue
			}
			seen[queueName] = struct{}{}
			names = append(names, queueName)
		}
	}
	sort.Strings(names)
	return names
}

func (registry *Registry) nextID() (string, error) {
	id := registry.ids.Add(1)
	suffix, err := randomHex(8)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("job-%d-%d-%s", core.Now().UnixNano(), id, suffix), nil
}

type queueEntry struct {
	name    string
	options QueueOptions
	code    []QueueOption
}

type workerEntry struct {
	name    string
	options WorkerOptions
	code    []WorkerOption
}

type jobEntry struct {
	name        string
	payloadType reflect.Type
	payloadName string
	options     JobOptions
	call        HandlerFunc
}

type workerStats struct {
	processed atomic.Int64
	succeeded atomic.Int64
	failed    atomic.Int64
	retried   atomic.Int64
}

func workerStatus(instances []WorkerInstance) string {
	if len(instances) == 0 {
		return "stopped"
	}
	return "running"
}
