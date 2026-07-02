package schedule

import (
	"context"
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/host"
	runlog "github.com/duxweb/runa/log"
	"github.com/duxweb/runa/task"
	"github.com/robfig/cron/v3"
)

// Option configures a schedule.
type Option func(*Options)

// Options stores schedule settings.
type Options struct {
	Mode           string
	Queue          string
	Timezone       string
	Enabled        bool
	SkipIfRunning  bool
	SkipIfQueued   bool
	DelayIfRunning bool
	Meta           core.Map
}

// Info describes one schedule.
type Info struct {
	Name     string
	Spec     string
	Task     string
	Payload  string
	Mode     string
	Queue    string
	Timezone string
	Enabled  bool
	Meta     core.Map
}

// Register registers a typed schedule.
func (registry *Registry) Register[T any](name string, spec string, taskName string, payload T, options ...Option) {
	if name == "" || spec == "" || taskName == "" {
		return
	}
	opts := Options{Mode: "direct", Enabled: true, Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	body, err := task.MarshalPayload(payload)
	if err != nil {
		return
	}
	payloadType := core.TypeOf[T]()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.entries[name] = append(registry.entries[name], entry{
		name:           name,
		spec:           spec,
		task:           taskName,
		payload:        body,
		payloadType:    payloadType,
		payloadName:    core.TypeName(payloadType),
		mode:           opts.Mode,
		queue:          opts.Queue,
		timezone:       opts.Timezone,
		enabled:        opts.Enabled,
		skipIfRunning:  opts.SkipIfRunning,
		skipIfQueued:   opts.SkipIfQueued,
		delayIfRunning: opts.DelayIfRunning,
		meta:           core.CloneMap(opts.Meta),
	})
}

// Freeze validates schedules.
func (registry *Registry) Freeze(tasks interface {
	PayloadType(string) (reflect.Type, bool)
}) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for name, entries := range registry.entries {
		if len(entries) != 1 {
			return fmt.Errorf("schedule %s already registered", name)
		}
		item := entries[0]
		payload, ok := tasks.PayloadType(item.task)
		if !ok {
			return fmt.Errorf("schedule %s task %s is not registered", name, item.task)
		}
		if payload != item.payloadType {
			return fmt.Errorf("schedule %s payload type mismatch: got %s want %s", name, item.payloadName, core.TypeName(payload))
		}
		if _, err := parseSpec(item); err != nil {
			return fmt.Errorf("schedule %s spec: %w", name, err)
		}
	}
	registry.frozen = true
	return nil
}

// Info returns schedule snapshots.
func (registry *Registry) Info() []Info {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]Info, 0, len(registry.entries))
	for name, entries := range registry.entries {
		if len(entries) == 0 {
			continue
		}
		item := entries[len(entries)-1]
		items = append(items, Info{Name: name, Spec: item.spec, Task: item.task, Payload: item.payloadName, Mode: item.mode, Queue: item.queue, Timezone: scheduleTimezone(item), Enabled: item.enabled, Meta: core.CloneMap(item.meta)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (registry *Registry) list() []entry {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	items := make([]entry, 0, len(registry.entries))
	for _, entries := range registry.entries {
		if len(entries) > 0 {
			items = append(items, entries[len(entries)-1])
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].name < items[j].name })
	return items
}

func scheduleTimezone(item entry) string {
	if item.timezone != "" {
		return item.timezone
	}
	if core.Location() == nil {
		return ""
	}
	return core.Location().String()
}

type entry struct {
	name           string
	spec           string
	task           string
	payload        []byte
	payloadType    reflect.Type
	payloadName    string
	mode           string
	queue          string
	timezone       string
	enabled        bool
	skipIfRunning  bool
	skipIfQueued   bool
	delayIfRunning bool
	meta           core.Map
}

// Unit runs scheduled jobs as a host unit.
type Unit struct {
	registry *Registry
	tasks    *task.Registry
	cron     *cron.Cron
	status   host.Status
	mu       sync.Mutex
	state    map[string]*runState
}

// NewUnit creates a schedule host unit.
func NewUnit(registry *Registry, tasks *task.Registry) *Unit {
	return &Unit{registry: registry, tasks: tasks, status: host.Created, state: make(map[string]*runState)}
}

func (unit *Unit) Name() string { return "schedule" }

// Start starts the scheduler.
func (unit *Unit) Start(ctx context.Context) error {
	unit.mu.Lock()
	if unit.status == host.Running {
		unit.mu.Unlock()
		return nil
	}
	unit.status = host.Starting
	unit.mu.Unlock()

	cronRunner := cron.New()
	for _, item := range unit.registry.list() {
		if !item.enabled {
			continue
		}
		spec, err := parseSpec(item)
		if err != nil {
			unit.setStatus(host.Failed)
			return err
		}
		item := item
		if _, err := cronRunner.AddFunc(spec, func() { unit.run(ctx, item) }); err != nil {
			unit.setStatus(host.Failed)
			return err
		}
	}
	unit.mu.Lock()
	unit.cron = cronRunner
	unit.status = host.Running
	unit.mu.Unlock()
	cronRunner.Start()
	return nil
}

// Stop stops the scheduler and waits for running jobs.
func (unit *Unit) Stop(ctx context.Context) error {
	unit.mu.Lock()
	cronRunner := unit.cron
	unit.status = host.Stopping
	unit.mu.Unlock()
	if cronRunner != nil {
		stopped := cronRunner.Stop()
		select {
		case <-stopped.Done():
		case <-ctx.Done():
			unit.setStatus(host.Failed)
			return ctx.Err()
		}
	}
	unit.setStatus(host.Stopped)
	return nil
}

// Status returns scheduler status.
func (unit *Unit) Status() host.Status {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	return unit.status
}

func (unit *Unit) run(ctx context.Context, item entry) {
	state := unit.runState(item.name)
	state.mu.Lock()
	if state.running {
		if item.skipIfRunning {
			state.mu.Unlock()
			return
		}
		if item.delayIfRunning {
			state.pending = true
			state.mu.Unlock()
			return
		}
	}
	state.running = true
	state.mu.Unlock()
	defer func() {
		if recovered := recover(); recovered != nil {
			runlog.Channel(nil, runlog.Schedule).ErrorContext(ctx, "schedule panic", "name", item.name, "task", item.task, "panic", recovered, "stack", string(debug.Stack()))
			state.mu.Lock()
			state.running = false
			state.pending = false
			state.mu.Unlock()
		}
	}()

	for {
		message := task.Message{Name: item.task, Payload: append([]byte(nil), item.payload...), Meta: core.CloneMap(item.meta)}
		if item.queue != "" {
			message.Queue = item.queue
			if item.skipIfQueued {
				message.Unique = "schedule:" + item.name
				message.UniqueStrategy = "until-done"
			}
		}
		if _, err := unit.tasks.DispatchMessage(ctx, message); err != nil {
			runlog.Channel(nil, runlog.Schedule).ErrorContext(ctx, "schedule task failed", "name", item.name, "task", item.task, "err", err)
		}

		state.mu.Lock()
		if !state.pending {
			state.running = false
			state.mu.Unlock()
			return
		}
		state.pending = false
		state.mu.Unlock()
	}
}

func (unit *Unit) runState(name string) *runState {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	state := unit.state[name]
	if state == nil {
		state = &runState{}
		unit.state[name] = state
	}
	return state
}

func (unit *Unit) setStatus(status host.Status) {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	unit.status = status
}

type runState struct {
	mu      sync.Mutex
	running bool
	pending bool
}

func parseSpec(item entry) (string, error) {
	spec := strings.TrimSpace(item.spec)
	if spec == "" {
		return "", fmt.Errorf("empty spec")
	}
	if duration, err := time.ParseDuration(spec); err == nil {
		return "@every " + duration.String(), nil
	}
	timezone := item.timezone
	if timezone == "" && core.Location() != nil {
		timezone = core.Location().String()
	}
	if timezone != "" && !strings.HasPrefix(spec, "@") && !strings.HasPrefix(spec, "CRON_TZ=") && !strings.HasPrefix(spec, "TZ=") {
		spec = "CRON_TZ=" + timezone + " " + spec
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(spec)
	return spec, err
}

// Direct uses direct task dispatch.
func Direct() Option {
	return func(options *Options) { options.Mode = "direct"; options.Queue = "" }
}

// Queue uses queue dispatch.
func Queue(name string) Option {
	return func(options *Options) { options.Mode = "queue"; options.Queue = name }
}

// Timezone sets schedule timezone.
func Timezone(name string) Option {
	return func(options *Options) { options.Timezone = name }
}

// Enabled sets whether schedule is enabled.
func Enabled(value bool) Option {
	return func(options *Options) { options.Enabled = value }
}

// SkipIfRunning skips a trigger while the previous run is active.
func SkipIfRunning() Option {
	return func(options *Options) { options.SkipIfRunning = true }
}

// SkipIfQueued skips queue dispatch while the previous queued schedule task is still pending or running.
func SkipIfQueued() Option {
	return func(options *Options) { options.SkipIfQueued = true }
}

// DelayIfRunning runs one delayed trigger after the active run completes.
func DelayIfRunning() Option {
	return func(options *Options) { options.DelayIfRunning = true }
}

// Meta sets schedule metadata.
func Meta(key string, value any) Option {
	return func(options *Options) {
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	}
}
