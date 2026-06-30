package view

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Context is the minimal route context used by renderers.
type Context interface {
	Context() context.Context
}

// Renderer loads and renders one view domain.
type Renderer interface {
	Load(ctx context.Context, set *Set) error
	Render(ctx Context, writer io.Writer, name string, data any) error
}

type reloadableRenderer interface {
	Load(ctx context.Context, set *Set) error
	ViewSet() Set
}

// RendererFunc adapts a function to Renderer.
type RendererFunc func(ctx Context, writer io.Writer, name string, data any) error

// Load is a no-op for RendererFunc.
func (RendererFunc) Load(context.Context, *Set) error { return nil }

// Render renders through fn.
func (fn RendererFunc) Render(ctx Context, writer io.Writer, name string, data any) error {
	return fn(ctx, writer, name, data)
}

// Set stores one named view domain.
type Set struct {
	Name         string
	Sources      []Source
	Funcs        map[string]any
	ContextFuncs map[string]func(context.Context) any
}

// HTML creates a standard html/template renderer.
func HTML(sources ...Source) Renderer {
	return &HTMLRenderer{
		set: Set{Sources: append([]Source(nil), sources...)},
	}
}

// HTMLRenderer renders templates with html/template.
type HTMLRenderer struct {
	set     Set
	funcs   template.FuncMap
	tpl     *template.Template
	files   map[string]File
	aliases map[string]string
	loaded  bool
	mu      sync.RWMutex
}

// Func registers a template function.
func (renderer *HTMLRenderer) Func(name string, fn any) *HTMLRenderer {
	if renderer.funcs == nil {
		renderer.funcs = make(template.FuncMap)
	}
	renderer.funcs[name] = fn
	return renderer
}

// Funcs registers template functions.
func (renderer *HTMLRenderer) Funcs(funcs template.FuncMap) *HTMLRenderer {
	if renderer.funcs == nil {
		renderer.funcs = make(template.FuncMap)
	}
	for name, fn := range funcs {
		renderer.funcs[name] = fn
	}
	return renderer
}

// Load scans sources and parses templates.
func (renderer *HTMLRenderer) Load(ctx context.Context, set *Set) error {
	if set != nil {
		renderer.set = *set
	}
	tpl := template.New(renderer.set.Name)
	if len(renderer.set.Funcs) > 0 {
		tpl = tpl.Funcs(template.FuncMap(renderer.set.Funcs))
	}
	if len(renderer.set.ContextFuncs) > 0 {
		tpl = tpl.Funcs(buildPlaceholderFuncMap(renderer.set.ContextFuncs))
	}
	if len(renderer.funcs) > 0 {
		tpl = tpl.Funcs(renderer.funcs)
	}
	files := make(map[string]File)
	aliases := make(map[string]string)
	for _, source := range renderer.set.Sources {
		items, err := Files(source)
		if err != nil {
			return err
		}
		for _, file := range items {
			body, err := fs.ReadFile(file.Source.FS, file.Path)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(file.Name)
			if existing := tpl.Lookup(name); existing != nil {
				_, _ = tpl.New(name).Parse("")
			}
			if _, err := tpl.New(name).Parse(string(body)); err != nil {
				return fmt.Errorf("parse template %s: %w", name, err)
			}
			files[name] = file
			aliases[name] = name
			withoutExt := strings.TrimSuffix(name, filepath.Ext(name))
			aliases[withoutExt] = name
		}
	}
	renderer.mu.Lock()
	renderer.tpl = tpl
	renderer.files = files
	renderer.aliases = aliases
	renderer.loaded = true
	renderer.mu.Unlock()
	return nil
}

// Render renders one template by name.
func (renderer *HTMLRenderer) Render(ctx Context, writer io.Writer, name string, data any) error {
	if err := renderer.ensureLoaded(ctx.Context()); err != nil {
		return err
	}
	if err := renderer.reloadIfChanged(ctx.Context()); err != nil {
		return err
	}
	renderer.mu.RLock()
	tpl := renderer.tpl
	aliases := renderer.aliases
	contextFuncs := cloneContextFuncs(renderer.set.ContextFuncs)
	templateName := aliases[normalizeName(name)]
	renderer.mu.RUnlock()
	if templateName == "" {
		return fmt.Errorf("template %s is not found", name)
	}
	if len(contextFuncs) > 0 {
		clone, err := tpl.Clone()
		if err != nil {
			return err
		}
		tpl = clone.Funcs(buildContextFuncMap(ctx.Context(), contextFuncs))
	}
	var buffer bytes.Buffer
	if err := tpl.ExecuteTemplate(&buffer, templateName, data); err != nil {
		return err
	}
	_, err := writer.Write(buffer.Bytes())
	return err
}

func (renderer *HTMLRenderer) ensureLoaded(ctx context.Context) error {
	renderer.mu.RLock()
	loaded := renderer.loaded
	renderer.mu.RUnlock()
	if loaded {
		return nil
	}
	return renderer.Load(ctx, &renderer.set)
}

func (renderer *HTMLRenderer) reloadIfChanged(ctx context.Context) error {
	renderer.mu.RLock()
	files := make(map[string]File, len(renderer.files))
	for name, file := range renderer.files {
		files[name] = file
	}
	sources := append([]Source(nil), renderer.set.Sources...)
	renderer.mu.RUnlock()
	if len(files) == 0 {
		return nil
	}
	for _, source := range sources {
		if !source.ReloadEnabled() {
			continue
		}
		items, err := Files(source)
		if err != nil {
			return err
		}
		if len(items) != countSourceFiles(files, source) {
			return renderer.Load(ctx, &renderer.set)
		}
		for _, item := range items {
			old := files[item.Name]
			if old.Size != item.Size || !sameModTime(old.ModTime, item.ModTime) {
				return renderer.Load(ctx, &renderer.set)
			}
		}
	}
	return nil
}

func countSourceFiles(files map[string]File, source Source) int {
	count := 0
	for _, file := range files {
		if file.Source.Root == source.Root {
			count++
		}
	}
	return count
}

func sameModTime(a time.Time, b time.Time) bool {
	return a.Equal(b) || a.Truncate(time.Second).Equal(b.Truncate(time.Second))
}

func cloneFuncs(funcs map[string]any) map[string]any {
	if len(funcs) == 0 {
		return nil
	}
	output := make(map[string]any, len(funcs))
	for name, fn := range funcs {
		output[name] = fn
	}
	return output
}

func cloneContextFuncs(funcs map[string]func(context.Context) any) map[string]func(context.Context) any {
	if len(funcs) == 0 {
		return nil
	}
	output := make(map[string]func(context.Context) any, len(funcs))
	for name, fn := range funcs {
		output[name] = fn
	}
	return output
}

func buildContextFuncMap(ctx context.Context, funcs map[string]func(context.Context) any) template.FuncMap {
	output := make(template.FuncMap, len(funcs))
	for name, build := range funcs {
		if build == nil {
			continue
		}
		output[name] = build(ctx)
	}
	return output
}

func buildPlaceholderFuncMap(funcs map[string]func(context.Context) any) template.FuncMap {
	output := make(template.FuncMap, len(funcs))
	for name := range funcs {
		output[name] = func(...any) any { return "" }
	}
	return output
}

func normalizeName(name string) string {
	name = filepath.ToSlash(strings.TrimPrefix(name, "./"))
	return strings.TrimPrefix(name, "/")
}
