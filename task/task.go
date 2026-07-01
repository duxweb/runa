package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"runtime/debug"
	"sort"
	"time"

	"github.com/duxweb/runa/core"
	runlog "github.com/duxweb/runa/log"
)

// MarshalPayload serializes a typed payload for task messages.
func MarshalPayload(payload any) ([]byte, error) {
	return json.Marshal(payload)
}

// TaskOf stores one task execution.
type TaskOf[T any] struct {
	ID      string
	Name    string
	Payload T
	Meta    core.Map
	Attempt int
}

// Handler executes a typed task.
type Handler[T any] func(ctx context.Context, task *TaskOf[T]) error

// TaskOption configures a registered task.
type TaskOption interface {
	ApplyTask(*Options)
}

// DispatchOption configures one dispatch call.
type DispatchOption interface {
	ApplyDispatch(*DispatchOptions)
}

// Options stores task settings.
type Options struct {
	Timeout time.Duration
	Retry   int
	Meta    core.Map
}

// DispatchOptions stores dispatch settings.
type DispatchOptions struct {
	Mode    string
	Queue   string
	Delay   time.Duration
	Timeout time.Duration
	Retry   int
	Unique  string
	Meta    core.Map
}

// Message is the serialized task dispatch format.
type Message struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Payload []byte        `json:"payload"`
	Meta    core.Map      `json:"meta"`
	Delay   time.Duration `json:"delay"`
	Timeout time.Duration `json:"timeout"`
	Retry   int           `json:"retry"`
	Unique  string        `json:"unique"`
	Queue   string        `json:"queue"`
	Attempt int           `json:"attempt"`
}

// Dispatcher dispatches a serialized task message.
type Dispatcher interface {
	Dispatch(ctx context.Context, message Message) (string, error)
}

// Info describes one registered task.
type Info struct {
	Name    string
	Payload string
	Timeout time.Duration
	Retry   int
	Source  string
	Meta    core.Map
}

// QueueDispatcher sets the dispatcher used by queued tasks.
func (registry *Registry) QueueDispatcher(dispatcher Dispatcher) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.queueDispatcher = dispatcher
}

// HasQueueDispatcher reports whether a queued-task dispatcher is configured.
func (registry *Registry) HasQueueDispatcher() bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.queueDispatcher != nil
}

// Register registers a typed task.
func (registry *Registry) Register[T any](name string, handler Handler[T], options ...TaskOption) {
	if name == "" || handler == nil {
		return
	}
	opts := Options{Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyTask(&opts)
		}
	}
	payloadType := core.TypeOf[T]()
	item := entry{
		name:        name,
		payloadType: payloadType,
		payloadName: core.TypeName(payloadType),
		timeout:     opts.Timeout,
		retry:       opts.Retry,
		meta:        core.CloneMap(opts.Meta),
		call: func(ctx context.Context, message Message) error {
			var payload T
			if len(message.Payload) > 0 {
				if err := json.Unmarshal(message.Payload, &payload); err != nil {
					return err
				}
			}
			return handler(ctx, &TaskOf[T]{ID: message.ID, Name: message.Name, Payload: payload, Meta: core.CloneMap(message.Meta), Attempt: message.Attempt})
		},
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.entries[name] = append(registry.entries[name], item)
}

// Dispatch dispatches a typed task.
func (registry *Registry) Dispatch[T any](ctx context.Context, name string, payload T, options ...DispatchOption) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	opts := DispatchOptions{Mode: "direct", Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyDispatch(&opts)
		}
	}
	return registry.DispatchMessage(ctx, Message{
		Name:    name,
		Payload: body,
		Meta:    core.CloneMap(opts.Meta),
		Delay:   opts.Delay,
		Timeout: opts.Timeout,
		Retry:   opts.Retry,
		Unique:  opts.Unique,
		Queue:   opts.Queue,
	})
}

// DispatchMessage dispatches a serialized task message.
func (registry *Registry) DispatchMessage(ctx context.Context, message Message) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if message.ID == "" {
		message.ID = registry.nextID()
	}
	entry, dispatcher, err := registry.entry(message.Name)
	if err != nil {
		return "", err
	}
	if message.Retry == 0 {
		message.Retry = entry.retry
	}
	if message.Timeout == 0 {
		message.Timeout = entry.timeout
	}
	if message.Meta == nil {
		message.Meta = make(core.Map)
	}
	if message.Queue != "" {
		if dispatcher == nil {
			return "", fmt.Errorf("task %s queue dispatcher is not configured", message.Name)
		}
		return dispatcher.Dispatch(ctx, message)
	}
	if message.Delay > 0 {
		timer := time.NewTimer(message.Delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return "", ctx.Err()
		}
	}
	if message.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, message.Timeout)
		defer cancel()
	}
	if err := executeTask(ctx, entry, message); err != nil {
		taskLogger().ErrorContext(ctx, "task failed", "task", message.Name, "id", message.ID, "attempt", message.Attempt, "err", err)
		return "", err
	}
	return message.ID, nil
}

func executeTask(ctx context.Context, entry entry, message Message) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("task panic: %v\n%s", recovered, debug.Stack())
		}
	}()
	return entry.call(ctx, message)
}

func taskLogger() *slog.Logger {
	return runlog.Channel(nil, runlog.Task)
}

// DispatchRaw dispatches a serialized task message.
func (registry *Registry) DispatchRaw(ctx context.Context, message Message) (string, error) {
	return registry.DispatchMessage(ctx, message)
}

// PayloadType returns a registered task payload type.
func (registry *Registry) PayloadType(name string) (reflect.Type, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entries := registry.entries[name]
	if len(entries) == 0 {
		return nil, false
	}
	return entries[len(entries)-1].payloadType, true
}

// Freeze validates the registry and marks it read-only.
func (registry *Registry) Freeze() error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for name, entries := range registry.entries {
		if len(entries) != 1 {
			return fmt.Errorf("task %s already registered", name)
		}
		if entries[0].payloadType == nil {
			return fmt.Errorf("task %s payload type is required", name)
		}
	}
	registry.frozen = true
	return nil
}

// Info returns task snapshots.
func (registry *Registry) Info() []Info {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]Info, 0, len(registry.entries))
	for name, entries := range registry.entries {
		if len(entries) == 0 {
			continue
		}
		item := entries[len(entries)-1]
		items = append(items, Info{Name: name, Payload: item.payloadName, Timeout: item.timeout, Retry: item.retry, Source: "app", Meta: core.CloneMap(item.meta)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (registry *Registry) entry(name string) (entry, Dispatcher, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entries := registry.entries[name]
	if len(entries) == 0 {
		return entry{}, nil, fmt.Errorf("task %s is not registered", name)
	}
	return entries[len(entries)-1], registry.queueDispatcher, nil
}

func (registry *Registry) nextID() string {
	id := registry.ids.Add(1)
	return fmt.Sprintf("task-%d-%d", core.Now().UnixNano(), id)
}

type entry struct {
	name        string
	payloadType reflect.Type
	payloadName string
	timeout     time.Duration
	retry       int
	meta        core.Map
	call        func(context.Context, Message) error
}

type taskOptionFunc func(*Options)

func (fn taskOptionFunc) ApplyTask(options *Options) { fn(options) }

type dispatchOptionFunc func(*DispatchOptions)

func (fn dispatchOptionFunc) ApplyDispatch(options *DispatchOptions) { fn(options) }

// Timeout sets task timeout.
func Timeout(duration time.Duration) timeoutOption { return timeoutOption{duration: duration} }

type timeoutOption struct{ duration time.Duration }

func (option timeoutOption) ApplyTask(options *Options) { options.Timeout = option.duration }
func (option timeoutOption) ApplyDispatch(options *DispatchOptions) {
	options.Timeout = option.duration
}

// Retry sets retry count.
func Retry(times int) retryOption { return retryOption{times: times} }

type retryOption struct{ times int }

func (option retryOption) ApplyTask(options *Options)             { options.Retry = option.times }
func (option retryOption) ApplyDispatch(options *DispatchOptions) { options.Retry = option.times }

// Direct uses direct dispatch.
func Direct() DispatchOption {
	return dispatchOptionFunc(func(options *DispatchOptions) {
		options.Mode = "direct"
		options.Queue = ""
	})
}

// Queue uses queue dispatch.
func Queue(name string) DispatchOption {
	return dispatchOptionFunc(func(options *DispatchOptions) {
		options.Mode = "queue"
		options.Queue = name
	})
}

// Delay sets dispatch delay.
func Delay(duration time.Duration) DispatchOption {
	return dispatchOptionFunc(func(options *DispatchOptions) { options.Delay = duration })
}

// Unique sets unique key.
func Unique(key string) DispatchOption {
	return dispatchOptionFunc(func(options *DispatchOptions) { options.Unique = key })
}

// Meta sets task or dispatch metadata.
func Meta(key string, value any) metaOption { return metaOption{key: key, value: value} }

type metaOption struct {
	key   string
	value any
}

func (option metaOption) ApplyTask(options *Options) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}

func (option metaOption) ApplyDispatch(options *DispatchOptions) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}
