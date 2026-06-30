package event

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/task"
)

// EventOf stores one emitted event.
type EventOf[T any] struct {
	Name    string
	Payload T
	Meta    core.Map
	Time    time.Time
}

// Listener handles an event payload.
type Listener[T any] func(ctx context.Context, event *EventOf[T]) error

// ListenerOption configures a listener.
type ListenerOption interface {
	ApplyEventListener(*ListenerOptions)
}

// EmitOption configures one emit call.
type EmitOption interface {
	ApplyEventEmit(*EmitOptions)
}

// ListenerOptions stores listener settings.
type ListenerOptions struct {
	Name     string
	Priority int
	Queue    string
	Meta     core.Map
}

// EmitOptions stores emit settings.
type EmitOptions struct {
	Meta core.Map
}

// Info describes one registered event listener.
type Info struct {
	Name     string
	Payload  string
	Listener string
	Priority int
	Queue    string
	Async    bool
	Source   string
	Meta     core.Map
}

// AsyncDispatcher receives async event listener messages.
type AsyncDispatcher interface {
	Dispatch(ctx context.Context, message task.Message) (string, error)
}

// Dispatcher sets the async dispatcher.
func (registry *Registry) Dispatcher(dispatcher AsyncDispatcher) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.dispatcher = dispatcher
}

// HasDispatcher reports whether an async dispatcher is configured.
func (registry *Registry) HasDispatcher() bool {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.dispatcher != nil
}

// On registers a typed event listener.
func (registry *Registry) On[T any](name string, listener Listener[T], options ...ListenerOption) {
	if name == "" || listener == nil {
		return
	}
	opts := ListenerOptions{Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyEventListener(&opts)
		}
	}
	payloadType := core.TypeOf[T]()
	entry := listenerEntry{
		event:       name,
		listener:    opts.Name,
		priority:    opts.Priority,
		queue:       opts.Queue,
		meta:        core.CloneMap(opts.Meta),
		payloadType: payloadType,
		payloadName: core.TypeName(payloadType),
		call: func(ctx context.Context, eventName string, payload any, meta core.Map) error {
			value, ok := payload.(T)
			if !ok {
				return fmt.Errorf("event %s payload type mismatch: got %T want %s", eventName, payload, core.TypeName(payloadType))
			}
			return listener(ctx, &EventOf[T]{Name: eventName, Payload: value, Meta: meta, Time: core.Now()})
		},
	}
	if entry.listener == "" {
		entry.listener = defaultListenerName(payloadType, len(registry.listeners[name])+1)
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.listeners[name] = append(registry.listeners[name], entry)
}

// Emit emits one typed event.
func (registry *Registry) Emit[T any](ctx context.Context, name string, payload T, options ...EmitOption) error {
	if ctx == nil {
		ctx = context.Background()
	}
	opts := EmitOptions{Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyEventEmit(&opts)
		}
	}
	entries, dispatcher := registry.entries(name)
	payloadType := core.TypeOf[T]()
	for _, entry := range entries {
		if entry.payloadType != payloadType {
			return fmt.Errorf("event %s payload type mismatch: got %s want %s", name, core.TypeName(payloadType), entry.payloadName)
		}
		if entry.queue != "" {
			if dispatcher == nil {
				return fmt.Errorf("event %s async dispatcher is not configured", name)
			}
			body, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = dispatcher.Dispatch(ctx, task.Message{
				Name:    "event:" + name + ":" + entry.listener,
				Payload: body,
				Queue:   entry.queue,
				Meta: core.Map{
					"event":    name,
					"listener": entry.listener,
					"meta":     core.CloneMap(opts.Meta),
				},
			})
			if err != nil {
				return err
			}
			continue
		}
		if err := entry.call(ctx, name, payload, core.CloneMap(opts.Meta)); err != nil {
			return err
		}
	}
	return nil
}

// DispatchMessage executes one queued event listener message.
func (registry *Registry) DispatchMessage(ctx context.Context, message task.Message) error {
	if ctx == nil {
		ctx = context.Background()
	}
	eventName, _ := message.Meta["event"].(string)
	listenerName, _ := message.Meta["listener"].(string)
	if eventName == "" || listenerName == "" {
		return fmt.Errorf("event message metadata is invalid")
	}
	entries, _ := registry.entries(eventName)
	var eventMeta core.Map
	if value, ok := message.Meta["meta"].(core.Map); ok {
		eventMeta = core.CloneMap(value)
	}
	for _, entry := range entries {
		if entry.listener != listenerName {
			continue
		}
		payload := reflect.New(entry.payloadType).Interface()
		if len(message.Payload) > 0 {
			if err := json.Unmarshal(message.Payload, payload); err != nil {
				return err
			}
		}
		return entry.call(ctx, eventName, reflect.ValueOf(payload).Elem().Interface(), eventMeta)
	}
	return fmt.Errorf("event %s listener %s is not registered", eventName, listenerName)
}

// Freeze validates the registry and marks it read-only.
func (registry *Registry) Freeze() error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for name, entries := range registry.listeners {
		var payload reflect.Type
		seen := map[string]struct{}{}
		for _, entry := range entries {
			if payload == nil {
				payload = entry.payloadType
			}
			if payload != entry.payloadType {
				return fmt.Errorf("event %s payload type conflict", name)
			}
			if entry.listener == "" {
				return fmt.Errorf("event %s listener name is required", name)
			}
			if _, ok := seen[entry.listener]; ok {
				return fmt.Errorf("event %s listener %s already registered", name, entry.listener)
			}
			seen[entry.listener] = struct{}{}
		}
		sortListeners(entries)
		registry.listeners[name] = entries
	}
	registry.frozen = true
	return nil
}

// Info returns listener snapshots.
func (registry *Registry) Info() []Info {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]Info, 0)
	for name, entries := range registry.listeners {
		for _, entry := range entries {
			items = append(items, Info{
				Name:     name,
				Payload:  entry.payloadName,
				Listener: entry.listener,
				Priority: entry.priority,
				Queue:    entry.queue,
				Async:    entry.queue != "",
				Source:   "app",
				Meta:     core.CloneMap(entry.meta),
			})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Priority > items[j].Priority
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func (registry *Registry) entries(name string) ([]listenerEntry, AsyncDispatcher) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	entries := append([]listenerEntry(nil), registry.listeners[name]...)
	sortListeners(entries)
	return entries, registry.dispatcher
}

type listenerEntry struct {
	event       string
	listener    string
	priority    int
	queue       string
	meta        core.Map
	payloadType reflect.Type
	payloadName string
	call        func(context.Context, string, any, core.Map) error
}

func sortListeners(entries []listenerEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].priority > entries[j].priority
	})
}

type listenerOptionFunc func(*ListenerOptions)

func (fn listenerOptionFunc) ApplyEventListener(options *ListenerOptions) { fn(options) }

type emitOptionFunc func(*EmitOptions)

func (fn emitOptionFunc) ApplyEventEmit(options *EmitOptions) { fn(options) }

// Priority sets listener priority.
func Priority(value int) ListenerOption {
	return listenerOptionFunc(func(options *ListenerOptions) { options.Priority = value })
}

// Queue marks the listener as async and sets the target queue.
func Queue(name string) ListenerOption {
	return listenerOptionFunc(func(options *ListenerOptions) { options.Queue = name })
}

// ListenerName sets listener name.
func ListenerName(name string) ListenerOption {
	return listenerOptionFunc(func(options *ListenerOptions) { options.Name = name })
}

// Meta sets listener metadata.
func Meta(key string, value any) metaOption { return metaOption{key: key, value: value} }

type metaOption struct {
	key   string
	value any
}

func (option metaOption) ApplyEventListener(options *ListenerOptions) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}

func (option metaOption) ApplyEventEmit(options *EmitOptions) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}

func defaultListenerName(payload reflect.Type, index int) string {
	name := core.TypeName(payload)
	if name == "" {
		name = "listener"
	}
	return fmt.Sprintf("%s#%d", name, index)
}
