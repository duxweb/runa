package queue

import (
	"context"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/event"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/task"
	"github.com/samber/do/v2"
)

type provider struct {
	runaprovider.Base
	drivers map[string]Driver
	queues  map[string][]QueueOption
	workers map[string][]WorkerOption
}

func Provider(options ...ProviderOption) runaprovider.Provider {
	item := &provider{
		drivers: make(map[string]Driver),
		queues:  make(map[string][]QueueOption),
		workers: make(map[string][]WorkerOption),
	}
	for _, option := range options {
		if option != nil {
			option(item)
		}
	}
	return item
}

func (provider *provider) Name() string { return "queue" }

func (provider *provider) Init(_ context.Context, ctx runaprovider.Context) error {
	runaprovider.ProvideDefault(ctx, func(do.Injector) (*Registry, error) { return New(), nil })
	return nil
}

func (provider *provider) Register(ctx runaprovider.Context) error {
	if err := ctx.RegisterCommand(commands()...); err != nil {
		return err
	}
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	store, err := runaprovider.Invoke[*config.Store](ctx)
	if err != nil {
		return err
	}
	for name, driver := range provider.drivers {
		registry.RegisterDriver(name, driver)
	}
	for name, options := range provider.queues {
		registry.Queue(name, options...)
	}
	for name, options := range provider.workers {
		registry.Worker(name, options...)
	}
	registry.Config(store)
	return nil
}

func (provider *provider) Boot(_ context.Context, ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*Registry](ctx)
	if err != nil {
		return err
	}
	tasks, taskErr := runaprovider.Invoke[*task.Registry](ctx)
	events, eventErr := runaprovider.Invoke[*event.Registry](ctx)
	dispatcher := taskDispatcher{queues: registry}
	if taskErr == nil {
		installTaskHandler(registry, tasks)
		if !tasks.HasQueueDispatcher() {
			tasks.QueueDispatcher(dispatcher)
		}
	}
	if eventErr == nil {
		installEventHandler(registry, events)
		if !events.HasDispatcher() {
			events.Dispatcher(dispatcher)
		}
	}
	return registry.Freeze()
}

type ProviderOption func(*provider)

func RegisterDriver(name string, driver Driver) ProviderOption {
	return func(provider *provider) {
		if name != "" && driver != nil {
			provider.drivers[name] = driver
		}
	}
}

func RegisterQueue(name string, options ...QueueOption) ProviderOption {
	return func(provider *provider) {
		if name != "" {
			provider.queues[name] = append([]QueueOption(nil), options...)
		}
	}
}

func RegisterWorker(name string, options ...WorkerOption) ProviderOption {
	return func(provider *provider) {
		if name != "" {
			provider.workers[name] = append([]WorkerOption(nil), options...)
		}
	}
}

func installTaskHandler(registry *Registry, tasks *task.Registry) {
	registry.Job[task.Message](InternalTaskJob, func(ctx context.Context, job *Job[task.Message]) error {
		message := job.Payload
		message.ID = job.ID
		message.Attempt = job.Attempt
		message.Queue = ""
		message.Delay = 0
		_, err := tasks.DispatchRaw(ctx, message)
		return err
	})
}

func installEventHandler(registry *Registry, events *event.Registry) {
	registry.Job[task.Message](InternalEventJob, func(ctx context.Context, job *Job[task.Message]) error {
		message := job.Payload
		message.ID = job.ID
		message.Attempt = job.Attempt
		return events.DispatchMessage(ctx, message)
	})
}

type taskDispatcher struct {
	queues *Registry
}

func (dispatcher taskDispatcher) Dispatch(ctx context.Context, message task.Message) (string, error) {
	queueName := message.Queue
	if queueName == "" {
		queueName = DefaultQueue
	}
	options := []PushOption{}
	if message.Delay > 0 {
		options = append(options, Delay(message.Delay))
	}
	if message.Timeout > 0 {
		options = append(options, Timeout(message.Timeout))
	}
	if message.Retry > 0 {
		options = append(options, Retry(message.Retry))
	}
	if message.Unique != "" {
		options = append(options, Unique(message.Unique))
	}
	if message.UniqueStrategy == string(UniqueStrategyUntilStart) {
		options = append(options, UniqueUntilStart())
	} else if message.UniqueStrategy == string(UniqueStrategyUntilDone) {
		options = append(options, UniqueUntilDone())
	}
	if message.UniqueTTL > 0 {
		options = append(options, UniqueFor(message.UniqueTTL))
	}
	for key, value := range message.Meta {
		options = append(options, Meta(key, value))
	}
	jobName := InternalTaskJob
	if isEventMessage(message) {
		jobName = InternalEventJob
	}
	return dispatcher.queues.Push(ctx, queueName, jobName, message, options...)
}

func isEventMessage(message task.Message) bool {
	if message.Meta == nil {
		return false
	}
	_, okEvent := message.Meta["event"]
	_, okListener := message.Meta["listener"]
	return okEvent && okListener
}

type queueConfig struct {
	Driver     string         `toml:"driver"`
	Workers    []string       `toml:"workers"`
	Retry      int            `toml:"retry"`
	RetryDelay time.Duration  `toml:"retry_delay"`
	Timeout    time.Duration  `toml:"timeout"`
	Retention  *time.Duration `toml:"retention"`
	Meta       core.Map       `toml:"meta"`
}

type workerConfig struct {
	Concurrency  int           `toml:"concurrency"`
	PollInterval time.Duration `toml:"poll_interval"`
	Lease        time.Duration `toml:"lease"`
	StopTimeout  time.Duration `toml:"stop_timeout"`
	Meta         core.Map      `toml:"meta"`
}

func configQueueOptions(store *config.Store, name string) []QueueOption {
	var item queueConfig
	ok, err := config.BindNamed(store, "queue", "queues", name, &item)
	if err != nil || !ok {
		return nil
	}
	options := make([]QueueOption, 0, 6+len(item.Meta))
	if item.Driver != "" {
		options = append(options, Use(item.Driver))
	}
	if len(item.Workers) > 0 {
		options = append(options, Workers(item.Workers...))
	}
	if item.Retry > 0 {
		options = append(options, Retry(item.Retry))
	}
	if item.RetryDelay > 0 {
		options = append(options, RetryDelay(item.RetryDelay))
	}
	if item.Timeout > 0 {
		options = append(options, Timeout(item.Timeout))
	}
	if item.Retention != nil {
		options = append(options, Retention(*item.Retention))
	}
	for key, value := range item.Meta {
		options = append(options, Meta(key, value))
	}
	return options
}

func configWorkerOptions(store *config.Store, name string) []WorkerOption {
	var item workerConfig
	ok, err := config.BindNamed(store, "queue", "workers", name, &item)
	if err != nil || !ok {
		return nil
	}
	options := make([]WorkerOption, 0, 5+len(item.Meta))
	if item.Concurrency > 0 {
		options = append(options, Concurrency(item.Concurrency))
	}
	if item.PollInterval > 0 {
		options = append(options, PollInterval(item.PollInterval))
	}
	if item.Lease > 0 {
		options = append(options, Lease(item.Lease))
	}
	if item.StopTimeout > 0 {
		options = append(options, StopTimeout(item.StopTimeout))
	}
	for key, value := range item.Meta {
		options = append(options, Meta(key, value))
	}
	return options
}
