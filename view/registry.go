package view

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/duxweb/runa/core"
	iregistry "github.com/duxweb/runa/kernel/registry"
	"github.com/duxweb/runa/view/internal/renderutil"
)

// Registry stores named view domains.
type Registry struct {
	entries      iregistry.Entries[*entry]
	funcs        map[string]any
	contextFuncs map[string]func(context.Context) any
}

// New creates a registry.
func New() *Registry {
	return &Registry{entries: iregistry.NewEntries[*entry]("")}
}

// Register adds or replaces a view renderer.
func (registry *Registry) Register(ctx context.Context, name string, renderer Renderer) error {
	if name == "" {
		return fmt.Errorf("view name is required")
	}
	if renderer == nil {
		return fmt.Errorf("view %s renderer is required", name)
	}
	set := &Set{Name: name}
	if withSet, ok := renderer.(interface{ ViewSet() Set }); ok {
		value := withSet.ViewSet()
		set.Sources = append([]Source(nil), value.Sources...)
		set.Funcs = renderutil.CloneFuncs(value.Funcs)
		set.ContextFuncs = renderutil.CloneContextFuncs(value.ContextFuncs)
	}
	mergeFuncs(set, registry.funcs, registry.contextFuncs)
	if err := renderer.Load(core.NormalizeContext(ctx), set); err != nil {
		return err
	}
	registry.entries.Register(name, &entry{name: name, renderer: renderer})
	return nil
}

// Func registers a static template helper for all subsequently registered view domains.
func (registry *Registry) Func(name string, fn any) *Registry {
	if registry == nil || name == "" || fn == nil {
		return registry
	}
	if registry.funcs == nil {
		registry.funcs = make(map[string]any)
	}
	registry.funcs[name] = fn
	registry.reloadAll(context.Background())
	return registry
}

// Funcs registers static template helpers for all subsequently registered view domains.
func (registry *Registry) Funcs(funcs map[string]any) *Registry {
	for name, fn := range funcs {
		registry.Func(name, fn)
	}
	return registry
}

// ContextFunc registers a request-scoped template helper for all subsequently registered view domains.
func (registry *Registry) ContextFunc(name string, build func(context.Context) any) *Registry {
	if registry == nil || name == "" || build == nil {
		return registry
	}
	if registry.contextFuncs == nil {
		registry.contextFuncs = make(map[string]func(context.Context) any)
	}
	registry.contextFuncs[name] = build
	registry.reloadAll(context.Background())
	return registry
}

// Render renders a named template from a domain.
func (registry *Registry) Render(ctx Context, writer io.Writer, domain string, name string, data any) error {
	item, ok := registry.entries.Entry(domain)
	if !ok || item == nil {
		if domain == "" {
			domain = registry.entries.Fallback()
		}
		return fmt.Errorf("view %s is not registered", domain)
	}
	return item.renderer.Render(ctx, writer, name, data)
}

// RenderString renders into a string.
func (registry *Registry) RenderString(ctx Context, domain string, name string, data any) (string, error) {
	var buffer bytes.Buffer
	if err := registry.Render(ctx, &buffer, domain, name, data); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

// Info returns view domain snapshots.
func (registry *Registry) Info() []Info {
	entries := registry.entries.All()
	fallback := registry.entries.Fallback()
	items := make([]Info, 0, len(entries))
	for name := range entries {
		items = append(items, Info{Name: name, Default: name == fallback})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// Info describes one view domain.
type Info struct {
	Name    string
	Default bool
}

type entry struct {
	name     string
	renderer Renderer
}

// ViewSet exposes renderer sources.
func (renderer *HTMLRenderer) ViewSet() Set {
	return renderer.set
}

func (registry *Registry) reloadAll(ctx context.Context) {
	if registry == nil {
		return
	}
	for _, item := range registry.entries.All() {
		if item == nil {
			continue
		}
		renderer, ok := item.renderer.(reloadableRenderer)
		if !ok {
			continue
		}
		set := renderer.ViewSet()
		mergeFuncs(&set, registry.funcs, registry.contextFuncs)
		_ = renderer.Load(core.NormalizeContext(ctx), &set)
	}
}

func mergeFuncs(set *Set, funcs map[string]any, contextFuncs map[string]func(context.Context) any) {
	if set == nil {
		return
	}
	if len(funcs) > 0 {
		if set.Funcs == nil {
			set.Funcs = make(map[string]any)
		}
		for name, fn := range funcs {
			if _, exists := set.Funcs[name]; !exists {
				set.Funcs[name] = fn
			}
		}
	}
	if len(contextFuncs) > 0 {
		if set.ContextFuncs == nil {
			set.ContextFuncs = make(map[string]func(context.Context) any)
		}
		for name, fn := range contextFuncs {
			if _, exists := set.ContextFuncs[name]; !exists {
				set.ContextFuncs[name] = fn
			}
		}
	}
}
