package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/host"
	runlog "github.com/duxweb/runa/log"
)

// Work starts a worker and blocks until ctx is canceled.
func (registry *Registry) Work(ctx context.Context, name string) error {
	ctx = core.NormalizeContext(ctx)
	if _, err := registry.worker(name); err != nil {
		return err
	}
	unit := NewUnit(registry, name)
	if err := unit.Start(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	return unit.Stop(context.Background())
}

// NewUnit creates a worker host unit.
func NewUnit(registry *Registry, name string) *Unit {
	return &Unit{registry: registry, name: name, status: host.Created}
}

// Unit runs one configured worker group.
type Unit struct {
	registry *Registry
	name     string
	status   host.Status
	cancel   context.CancelFunc
	done     chan struct{}
	busy     atomic.Int64
	mu       sync.Mutex
}

func (unit *Unit) Name() string { return "queue:" + unit.name }

// Start starts the worker loop.
func (unit *Unit) Start(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	unit.mu.Lock()
	if unit.status == host.Running || unit.status == host.Starting {
		unit.mu.Unlock()
		return nil
	}
	unit.status = host.Starting
	unit.done = make(chan struct{})
	runCtx, cancel := context.WithCancel(ctx)
	unit.cancel = cancel
	unit.mu.Unlock()

	if _, err := unit.registry.worker(unit.name); err != nil {
		unit.setStatus(host.Failed)
		cancel()
		return err
	}
	go unit.loop(runCtx)
	unit.setStatus(host.Running)
	return nil
}

// Drain stops reserving new jobs.
func (unit *Unit) Drain(ctx context.Context) error {
	unit.mu.Lock()
	cancel := unit.cancel
	unit.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// Stop stops the worker and waits for running jobs.
func (unit *Unit) Stop(ctx context.Context) error {
	ctx = core.NormalizeContext(ctx)
	if worker, err := unit.registry.worker(unit.name); err == nil && worker.options.StopTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, worker.options.StopTimeout)
		defer cancel()
	}
	unit.mu.Lock()
	cancel := unit.cancel
	done := unit.done
	unit.status = host.Stopping
	unit.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			unit.setStatus(host.Failed)
			return ctx.Err()
		}
	}
	unit.setStatus(host.Stopped)
	return nil
}

// Status returns current worker status.
func (unit *Unit) Status() host.Status {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	return unit.status
}

func (unit *Unit) loop(ctx context.Context) {
	defer close(unit.done)
	defer unit.setStatus(host.Stopped)

	worker, err := unit.registry.worker(unit.name)
	if err != nil {
		unit.setStatus(host.Failed)
		return
	}
	instance := unit.instance(worker)
	stateDrivers := unit.stateDrivers(worker)
	for _, driver := range stateDrivers {
		_ = driver.Register(ctx, instance)
	}
	defer func() {
		for _, driver := range stateDrivers {
			_ = driver.Unregister(context.Background(), instance.ID)
		}
	}()

	heartbeatDone := make(chan struct{})
	go unit.heartbeat(ctx, stateDrivers, instance.ID, heartbeatDone)
	defer func() {
		close(heartbeatDone)
	}()

	sem := make(chan struct{}, worker.options.Concurrency)
	var wait sync.WaitGroup
	defer wait.Wait()
	queueNames := unit.registry.workerQueueNames(worker.name)
	sweepDone := make(chan struct{})
	go unit.sweep(ctx, queueNames, sweepDone)
	defer close(sweepDone)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		dispatched := false
		for _, queueName := range queueNames {
			free := reserveSlots(ctx, sem, worker.options.Concurrency)
			if free == 0 {
				return
			}
			messages, driver, err := unit.reserve(ctx, queueName, free, worker.options.Lease)
			if err != nil || len(messages) == 0 {
				releaseSlots(sem, free)
				continue
			}
			dispatched = true
			unused := free - len(messages)
			if unused > 0 {
				releaseSlots(sem, unused)
			}
			for _, message := range messages {
				wait.Add(1)
				unit.busy.Add(1)
				go func(message *JobMessage, driver Driver) {
					defer wait.Done()
					defer unit.busy.Add(-1)
					defer func() { <-sem }()
					unit.run(ctx, worker, message, driver)
				}(message, driver)
			}
		}
		if !dispatched {
			timer := time.NewTimer(worker.options.PollInterval)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			}
		}
	}
}

func (unit *Unit) reserve(ctx context.Context, queueName string, limit int, lease time.Duration) ([]*JobMessage, Driver, error) {
	_, driver, err := unit.registry.queue(queueName)
	if err != nil {
		return nil, nil, err
	}
	items, err := driver.Reserve(ctx, queueName, limit, lease)
	return items, driver, err
}

func (unit *Unit) run(ctx context.Context, worker workerEntry, message *JobMessage, driver Driver) {
	stats := unit.registry.workerStats(worker.name)
	stats.processed.Add(1)
	err := unit.execute(ctx, worker, message, driver)
	if err == nil {
		if ackErr := driver.Ack(context.Background(), message.Queue, message.ID); ackErr != nil {
			stats.failed.Add(1)
			queueLogger().ErrorContext(ctx, "queue job ack failed", "worker", worker.name, "queue", message.Queue, "job", message.Name, "id", message.ID, "attempt", message.Attempt, "err", ackErr)
		} else {
			stats.succeeded.Add(1)
		}
		return
	}
	if message.Attempt <= message.MaxAttempt {
		delay := message.RetryDelay
		if delay <= 0 {
			delay = DefaultRetryDelay
		}
		if releaseErr := driver.Release(context.Background(), message.Queue, message.ID, delay, err.Error()); releaseErr != nil {
			stats.failed.Add(1)
			queueLogger().ErrorContext(ctx, "queue job retry failed", "worker", worker.name, "queue", message.Queue, "job", message.Name, "id", message.ID, "attempt", message.Attempt, "err", releaseErr)
			return
		}
		stats.retried.Add(1)
		queueLogger().WarnContext(ctx, "queue job failed; retry scheduled", "worker", worker.name, "queue", message.Queue, "job", message.Name, "id", message.ID, "attempt", message.Attempt, "max_attempt", message.MaxAttempt, "retry_delay", delay, "err", err)
		return
	}
	if failErr := driver.Fail(context.Background(), message.Queue, message.ID, err.Error()); failErr != nil {
		stats.failed.Add(1)
		queueLogger().ErrorContext(ctx, "queue job fail mark failed", "worker", worker.name, "queue", message.Queue, "job", message.Name, "id", message.ID, "attempt", message.Attempt, "err", failErr)
		return
	}
	stats.failed.Add(1)
	queueLogger().ErrorContext(ctx, "queue job failed", "worker", worker.name, "queue", message.Queue, "job", message.Name, "id", message.ID, "attempt", message.Attempt, "max_attempt", message.MaxAttempt, "err", err)
}

func (unit *Unit) execute(ctx context.Context, worker workerEntry, message *JobMessage, driver Driver) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("queue job panic: %v", recovered)
		}
	}()
	entry, err := unit.registry.job(message.Name)
	if err != nil {
		return err
	}
	jobCtx := ctx
	cancel := func() {}
	if message.Timeout > 0 {
		jobCtx, cancel = context.WithTimeout(ctx, message.Timeout)
	}
	defer cancel()

	stopRenew := func() {}
	if worker.options.Lease > 0 {
		stopRenew = renewLease(jobCtx, driver, message.Queue, message.ID, worker.options.Lease)
	}
	defer stopRenew()
	call := entry.call
	for i := len(worker.options.Middlewares) - 1; i >= 0; i-- {
		call = worker.options.Middlewares[i](call)
	}
	middlewares := unit.registry.middlewareSnapshot()
	for i := len(middlewares) - 1; i >= 0; i-- {
		call = middlewares[i](call)
	}
	return call(jobCtx, message)
}

func (unit *Unit) instance(worker workerEntry) WorkerInstance {
	hostname, _ := os.Hostname()
	now := core.Now()
	return WorkerInstance{
		ID:          fmt.Sprintf("%s-%s-%d-%d", worker.name, hostname, os.Getpid(), now.UnixNano()),
		Worker:      worker.name,
		Hostname:    hostname,
		PID:         os.Getpid(),
		Queues:      unit.registry.workerQueueNames(worker.name),
		Concurrency: worker.options.Concurrency,
		Busy:        int(unit.busy.Load()),
		Status:      "running",
		StartedAt:   now,
		HeartbeatAt: now,
	}
}

func (unit *Unit) stateDrivers(worker workerEntry) []WorkerState {
	seen := map[Driver]struct{}{}
	var items []WorkerState
	for _, queueName := range unit.registry.workerQueueNames(worker.name) {
		_, driver, err := unit.registry.queue(queueName)
		if err != nil {
			continue
		}
		if _, ok := seen[driver]; ok {
			continue
		}
		seen[driver] = struct{}{}
		if state, ok := driver.(WorkerState); ok {
			items = append(items, state)
		}
	}
	return items
}

func (unit *Unit) heartbeat(ctx context.Context, drivers []WorkerState, id string, done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, driver := range drivers {
				_ = driver.Heartbeat(ctx, id)
			}
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (unit *Unit) sweep(ctx context.Context, queueNames []string, done <-chan struct{}) {
	retentions := make(map[string]time.Duration, len(queueNames))
	interval := time.Minute
	for _, queueName := range queueNames {
		entry, _, err := unit.registry.queue(queueName)
		if err != nil || entry.options.Retention <= 0 {
			continue
		}
		retentions[queueName] = entry.options.Retention
		if entry.options.Retention < interval {
			interval = entry.options.Retention
		}
	}
	if len(retentions) == 0 {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			unit.purgeFailed(ctx, retentions, interval)
		case <-done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (unit *Unit) purgeFailed(ctx context.Context, retentions map[string]time.Duration, lockTTL time.Duration) {
	for queueName, retention := range retentions {
		_, driver, err := unit.registry.queue(queueName)
		if err != nil {
			continue
		}
		if locker, ok := driver.(SweepLocker); ok {
			locked, err := locker.LockSweep(ctx, queueName, lockTTL)
			if err != nil {
				queueLogger().WarnContext(ctx, "queue failed job purge lock failed", "queue", queueName, "retention", retention, "err", err)
				continue
			}
			if !locked {
				continue
			}
		}
		olderThan := core.Now().Add(-retention)
		count, err := driver.Purge(ctx, queueName, StateFailed, olderThan)
		if err != nil {
			if errors.Is(err, ErrUnsupported) {
				continue
			}
			queueLogger().WarnContext(ctx, "queue failed job purge failed", "queue", queueName, "retention", retention, "err", err)
			continue
		}
		if count > 0 {
			queueLogger().DebugContext(ctx, "queue failed jobs purged", "queue", queueName, "count", count, "retention", retention)
		}
	}
}

func (unit *Unit) setStatus(status host.Status) {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	unit.status = status
}

func queueLogger() *slog.Logger {
	return runlog.Channel(nil, runlog.Queue)
}

func reserveSlots(ctx context.Context, sem chan struct{}, limit int) int {
	free := 0
	for free < limit {
		select {
		case sem <- struct{}{}:
			free++
		default:
			if free > 0 {
				return free
			}
			select {
			case sem <- struct{}{}:
				return 1
			case <-ctx.Done():
				return 0
			}
		}
	}
	return free
}

func releaseSlots(sem chan struct{}, count int) {
	for range count {
		<-sem
	}
}

func renewLease(ctx context.Context, driver Driver, queueName string, id string, lease time.Duration) func() {
	interval := lease / 2
	if interval <= 0 {
		interval = time.Second
	}
	var mu sync.Mutex
	stopped := false
	var timer *time.Timer
	var renew func()
	renew = func() {
		mu.Lock()
		if stopped {
			mu.Unlock()
			return
		}
		mu.Unlock()
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = driver.Renew(ctx, queueName, id, lease)
		mu.Lock()
		defer mu.Unlock()
		if !stopped {
			timer = time.AfterFunc(interval, renew)
		}
	}
	timer = time.AfterFunc(interval, renew)
	return func() {
		mu.Lock()
		defer mu.Unlock()
		stopped = true
		if timer != nil {
			timer.Stop()
		}
	}
}
