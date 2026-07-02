package queue

import (
	"time"

	"github.com/duxweb/runa/core"
)

// QueueOption configures a queue.
type QueueOption interface {
	ApplyQueue(*QueueOptions)
}

// WorkerOption configures a worker.
type WorkerOption interface {
	ApplyWorker(*WorkerOptions)
}

// JobOption configures a registered job.
type JobOption interface {
	ApplyJob(*JobOptions)
}

// PushOption configures one push call.
type PushOption interface {
	ApplyPush(*PushOptions)
}

// QueueOptions stores queue settings.
type QueueOptions struct {
	Driver       string
	Workers      []string
	Retry        int
	RetryDelay   time.Duration
	Timeout      time.Duration
	Retention    time.Duration
	Meta         core.Map
	retentionSet bool
}

// WorkerOptions stores worker settings.
type WorkerOptions struct {
	Concurrency  int
	PollInterval time.Duration
	Lease        time.Duration
	StopTimeout  time.Duration
	Middlewares  []Middleware
	Meta         core.Map
}

// JobOptions stores registered job settings.
type JobOptions struct {
	Retry      int
	RetryDelay time.Duration
	Timeout    time.Duration
	Meta       core.Map
}

// PushOptions stores one push settings.
type PushOptions struct {
	Delay          time.Duration
	Retry          int
	RetryDelay     time.Duration
	Timeout        time.Duration
	Unique         string
	UniqueStrategy string
	UniqueTTL      time.Duration
	Meta           core.Map
}

type optionFunc func(*QueueOptions)

func (fn optionFunc) ApplyQueue(options *QueueOptions) { fn(options) }

type workerOptionFunc func(*WorkerOptions)

func (fn workerOptionFunc) ApplyWorker(options *WorkerOptions) { fn(options) }

type jobOptionFunc func(*JobOptions)

func (fn jobOptionFunc) ApplyJob(options *JobOptions) { fn(options) }

type pushOptionFunc func(*PushOptions)

func (fn pushOptionFunc) ApplyPush(options *PushOptions) { fn(options) }

// Driver sets the queue storage driver name.
func Use(name string) QueueOption {
	return optionFunc(func(options *QueueOptions) { options.Driver = name })
}

// Retry sets max retry count.
func Retry(times int) retryOption { return retryOption{times: times} }

type retryOption struct{ times int }

func (option retryOption) ApplyQueue(options *QueueOptions) { options.Retry = option.times }
func (option retryOption) ApplyJob(options *JobOptions)     { options.Retry = option.times }
func (option retryOption) ApplyPush(options *PushOptions)   { options.Retry = option.times }

// RetryDelay sets retry delay.
func RetryDelay(duration time.Duration) retryDelayOption { return retryDelayOption{duration: duration} }

type retryDelayOption struct{ duration time.Duration }

func (option retryDelayOption) ApplyQueue(options *QueueOptions) {
	options.RetryDelay = option.duration
}
func (option retryDelayOption) ApplyJob(options *JobOptions)   { options.RetryDelay = option.duration }
func (option retryDelayOption) ApplyPush(options *PushOptions) { options.RetryDelay = option.duration }

// Timeout sets job execution timeout.
func Timeout(duration time.Duration) timeoutOption { return timeoutOption{duration: duration} }

type timeoutOption struct{ duration time.Duration }

func (option timeoutOption) ApplyQueue(options *QueueOptions) { options.Timeout = option.duration }
func (option timeoutOption) ApplyJob(options *JobOptions)     { options.Timeout = option.duration }
func (option timeoutOption) ApplyPush(options *PushOptions)   { options.Timeout = option.duration }

// Retention sets failed job retention.
func Retention(duration time.Duration) QueueOption {
	return optionFunc(func(options *QueueOptions) {
		options.Retention = duration
		options.retentionSet = true
	})
}

// Workers binds this queue to worker names.
func Workers(names ...string) QueueOption {
	return optionFunc(func(options *QueueOptions) {
		options.Workers = cleanNames(names)
	})
}

// Concurrency sets worker concurrency.
func Concurrency(value int) WorkerOption {
	return workerOptionFunc(func(options *WorkerOptions) { options.Concurrency = value })
}

// PollInterval sets worker idle polling interval.
func PollInterval(duration time.Duration) WorkerOption {
	return workerOptionFunc(func(options *WorkerOptions) { options.PollInterval = duration })
}

// Lease sets reserved job lease duration.
func Lease(duration time.Duration) WorkerOption {
	return workerOptionFunc(func(options *WorkerOptions) { options.Lease = duration })
}

// StopTimeout sets worker graceful stop timeout.
func StopTimeout(duration time.Duration) WorkerOption {
	return workerOptionFunc(func(options *WorkerOptions) { options.StopTimeout = duration })
}

// UseMiddleware adds worker job execution middleware.
func UseMiddleware(middlewares ...Middleware) WorkerOption {
	return workerOptionFunc(func(options *WorkerOptions) {
		for _, middleware := range middlewares {
			if middleware != nil {
				options.Middlewares = append(options.Middlewares, middleware)
			}
		}
	})
}

// Delay sets push delay.
func Delay(duration time.Duration) PushOption {
	return pushOptionFunc(func(options *PushOptions) { options.Delay = duration })
}

// Unique sets push unique key.
func Unique(key string) PushOption {
	return pushOptionFunc(func(options *PushOptions) {
		options.Unique = key
		if options.UniqueStrategy == "" {
			options.UniqueStrategy = string(UniqueStrategyUntilDone)
		}
	})
}

// UniqueUntilStart releases the unique lock when a worker reserves the job.
func UniqueUntilStart() PushOption {
	return pushOptionFunc(func(options *PushOptions) { options.UniqueStrategy = string(UniqueStrategyUntilStart) })
}

// UniqueUntilDone releases the unique lock when the job succeeds or reaches terminal failure.
func UniqueUntilDone() PushOption {
	return pushOptionFunc(func(options *PushOptions) { options.UniqueStrategy = string(UniqueStrategyUntilDone) })
}

// UniqueFor sets a maximum unique lock lifetime.
func UniqueFor(ttl time.Duration) PushOption {
	return pushOptionFunc(func(options *PushOptions) { options.UniqueTTL = ttl })
}

// Meta sets metadata.
func Meta(key string, value any) metaOption { return metaOption{key: key, value: value} }

type metaOption struct {
	key   string
	value any
}

func (option metaOption) ApplyQueue(options *QueueOptions) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}

func (option metaOption) ApplyWorker(options *WorkerOptions) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}

func (option metaOption) ApplyJob(options *JobOptions) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}

func (option metaOption) ApplyPush(options *PushOptions) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}

func cleanNames(names []string) []string {
	seen := map[string]struct{}{}
	values := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		values = append(values, name)
	}
	return values
}
