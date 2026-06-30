package log

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sort"
	"sync"
)

// Registry stores named log channel configuration and instances.
type Registry struct {
	outputs map[string][]Output
	loggers map[string]*slog.Logger
	mu      sync.RWMutex
}

// Info stores one log channel snapshot.
type Info struct {
	Name    string
	Outputs int
	Default bool
}

// New creates a registry.
func New() *Registry {
	registry := &Registry{
		outputs: make(map[string][]Output),
		loggers: make(map[string]*slog.Logger),
	}
	registry.Set(DefaultName, Console())
	return registry
}

// Set registers a named logger channel.
func (registry *Registry) Set(name string, outputs ...Output) {
	if name == "" {
		name = DefaultName
	}
	if len(outputs) == 0 {
		outputs = []Output{Discard()}
	}
	registry.mu.Lock()
	registry.outputs[name] = append([]Output(nil), outputs...)
	delete(registry.loggers, name)
	registry.mu.Unlock()
}

// Get returns a named logger.
func (registry *Registry) Get(name string) *slog.Logger {
	if name == "" {
		name = DefaultName
	}
	registry.mu.RLock()
	if logger := registry.loggers[name]; logger != nil {
		registry.mu.RUnlock()
		return logger
	}
	registry.mu.RUnlock()
	return registry.Build(context.Background(), name)
}

// Info returns configured log channel snapshots.
func (registry *Registry) Info() []Info {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	names := make([]string, 0, len(registry.outputs))
	for name := range registry.outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]Info, 0, len(names))
	for _, name := range names {
		items = append(items, Info{Name: name, Outputs: len(registry.outputs[name]), Default: name == DefaultName})
	}
	return items
}

// Build builds a named logger.
func (registry *Registry) Build(ctx context.Context, name string) *slog.Logger {
	if name == "" {
		name = DefaultName
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if logger := registry.loggers[name]; logger != nil {
		return logger
	}
	outputs := registry.outputs[name]
	if len(outputs) == 0 && name != DefaultName {
		outputs = registry.outputs[DefaultName]
	}
	if len(outputs) == 0 {
		outputs = []Output{Discard()}
	}
	handlers := make([]slog.Handler, 0, len(outputs))
	var buildErr error
	for _, output := range outputs {
		if output == nil {
			continue
		}
		handler, err := output.Build(ctx, name)
		if err == nil && handler != nil {
			handlers = append(handlers, handler)
			continue
		}
		if err != nil {
			buildErr = errors.Join(buildErr, err)
		}
	}
	if buildErr != nil {
		fmt.Fprintf(os.Stderr, "runa log output build failed channel=%s error=%v\n", name, buildErr)
		if len(handlers) == 0 {
			handlers = append(handlers, slog.NewTextHandler(os.Stderr, nil))
		}
	}
	logger := slog.New(fanout(handlers...)).With("channel", name)
	registry.loggers[name] = logger
	return logger
}

// Close closes closeable log outputs once.
func (registry *Registry) Close(ctx context.Context) error {
	registry.mu.RLock()
	outputs := make(map[string][]Output, len(registry.outputs))
	for name, items := range registry.outputs {
		outputs[name] = append([]Output(nil), items...)
	}
	registry.mu.RUnlock()

	seen := make(map[any]struct{})
	var joined error
	for channel, items := range outputs {
		for _, output := range items {
			closer, ok := output.(closeableOutput)
			if !ok || closer == nil {
				continue
			}
			if key, ok := outputKey(closer); ok {
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
			}
			if err := closer.Close(ctx); err != nil {
				joined = errors.Join(joined, fmt.Errorf("close log output %s: %w", channel, err))
			}
		}
	}
	return joined
}

// Shutdown closes closeable log outputs when managed by DI.
func (registry *Registry) Shutdown(ctx context.Context) error {
	return registry.Close(ctx)
}

func outputKey(value any) (any, bool) {
	if output, ok := value.(writerOutput); ok {
		return writerKey(output.writer)
	}
	reflected := reflect.ValueOf(value)
	if reflected.IsValid() && reflected.Type().Comparable() {
		return value, true
	}
	return nil, false
}

func writerKey(writer any) (any, bool) {
	reflected := reflect.ValueOf(writer)
	if reflected.IsValid() && reflected.Type().Comparable() {
		return writer, true
	}
	return nil, false
}
