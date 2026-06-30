package route

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/duxweb/runa/config"
)

// Registry stores routes and builds router runtimes.
type Registry struct {
	mu         sync.Mutex
	routes     []*Route
	index      map[string]int
	domains    map[string]*Group
	groups     map[string]*Group
	root       *Group
	mounts     []routeMount
	errorsList []error
	runtime    http.Handler
	errors     ErrorPipeline
	envelope   Envelope
	lang       Lang
	translator Translator
	config     *config.Store
	services   map[string]any
}

// New creates a registry.
func New() *Registry {
	registry := &Registry{
		index:   make(map[string]int),
		domains: make(map[string]*Group),
		groups:  make(map[string]*Group),
	}
	registry.root = NewGroup(registry, "")
	return registry
}

// RouteGroup returns the root route group.
func (registry *Registry) RouteGroup() *Group { return registry.root }

// Use appends root group middlewares.
func (registry *Registry) Use(middlewares ...Middleware) *Registry {
	registry.root.Use(middlewares...)
	return registry
}

// RegisterDomain registers a named route domain.
func (registry *Registry) RegisterDomain(name string, group *Group) error {
	if registry == nil || name == "" || group == nil {
		return nil
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.domains == nil {
		registry.domains = make(map[string]*Group)
	}
	if _, exists := registry.domains[name]; exists {
		return fmt.Errorf("route domain %s is already registered", name)
	}
	registry.domains[name] = group
	return nil
}

// Domain registers a named route domain.
func (registry *Registry) Domain(name string, prefix string, fn ...func(*Group)) *Group {
	domain := registry.root.Group(prefix).Name(name)
	if err := registry.RegisterDomain(name, domain); err != nil {
		registry.errorsList = append(registry.errorsList, err)
	}
	for _, callback := range fn {
		if callback != nil {
			callback(domain)
		}
	}
	return domain
}

// GetDomain returns a named route domain.
func (registry *Registry) GetDomain(name string) *Group {
	if registry == nil || name == "" {
		return nil
	}
	registry.mu.Lock()
	group := registry.domains[name]
	registry.mu.Unlock()
	return group
}

// RegisterGroup registers a named route group.
func (registry *Registry) RegisterGroup(name string, group *Group) {
	if registry == nil || name == "" || group == nil {
		return
	}
	registry.mu.Lock()
	if registry.groups == nil {
		registry.groups = make(map[string]*Group)
	}
	registry.groups[name] = group
	registry.mu.Unlock()
}

// GetGroup returns a named route group.
func (registry *Registry) GetGroup(name string) *Group {
	if registry == nil || name == "" {
		return nil
	}
	registry.mu.Lock()
	group := registry.groups[name]
	registry.mu.Unlock()
	return group
}

// Group creates a route group from the root group.
func (registry *Registry) Group(prefix string, fn ...func(*Group)) *Group {
	return registry.root.Group(prefix, fn...)
}

// MountDomain mounts routes into a named route domain, resolving missing domains later.
func (registry *Registry) MountDomain(name string, fn func(*Group)) {
	if name == "" || fn == nil {
		return
	}
	if domain := registry.GetDomain(name); domain != nil {
		fn(domain)
		return
	}
	registry.mounts = append(registry.mounts, routeMount{kind: "domain", name: name, register: fn})
}

// MountGroup mounts routes into a named route group, resolving missing groups later.
func (registry *Registry) MountGroup(name string, fn func(*Group)) {
	if name == "" || fn == nil {
		return
	}
	if group := registry.GetGroup(name); group != nil {
		fn(group)
		return
	}
	registry.mounts = append(registry.mounts, routeMount{kind: "group", name: name, register: fn})
}

// ResolveMounts resolves deferred mounts.
func (registry *Registry) ResolveMounts() error {
	for _, mount := range registry.mounts {
		var group *Group
		if mount.kind == "domain" {
			group = registry.GetDomain(mount.name)
			if group == nil {
				return fmt.Errorf("route domain %s is not registered", mount.name)
			}
		} else {
			group = registry.GetGroup(mount.name)
			if group == nil {
				return fmt.Errorf("route group %s is not registered", mount.name)
			}
		}
		mount.register(group)
	}
	registry.mounts = nil
	return nil
}

// PendingError returns route registration errors.
func (registry *Registry) PendingError() error { return errors.Join(registry.errorsList...) }

type routeMount struct {
	kind     string
	name     string
	register func(*Group)
}

func (registry *Registry) Get(path string, handler Handler) *Route {
	return registry.root.Get(path, handler)
}
func (registry *Registry) Post(path string, handler Handler) *Route {
	return registry.root.Post(path, handler)
}
func (registry *Registry) Put(path string, handler Handler) *Route {
	return registry.root.Put(path, handler)
}
func (registry *Registry) Patch(path string, handler Handler) *Route {
	return registry.root.Patch(path, handler)
}
func (registry *Registry) Delete(path string, handler Handler) *Route {
	return registry.root.Delete(path, handler)
}
func (registry *Registry) Options(path string, handler Handler) *Route {
	return registry.root.Options(path, handler)
}
func (registry *Registry) Head(path string, handler Handler) *Route {
	return registry.root.Head(path, handler)
}
func (registry *Registry) Any(path string, handler Handler) *Route {
	return registry.root.Any(path, handler)
}

// OnError sets the app-level error handler.
func (registry *Registry) OnError(handler ErrorHandler) {
	registry.mu.Lock()
	registry.errors.OnError = handler
	registry.runtime = nil
	registry.mu.Unlock()
}

// Error sets the app-level error renderer.
func (registry *Registry) Error(renderer ErrorRenderer) {
	registry.mu.Lock()
	registry.errors.Renderer = renderer
	registry.runtime = nil
	registry.mu.Unlock()
}

// ErrorPipeline returns the app-level error pipeline.
func (registry *Registry) ErrorPipeline() ErrorPipeline {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return registry.errors
}

// Envelope sets the app-level response envelope.
func (registry *Registry) Envelope(envelope Envelope) {
	registry.mu.Lock()
	registry.envelope = envelope
	registry.runtime = nil
	registry.mu.Unlock()
}

// EnvelopeDef returns the app-level response envelope.
func (registry *Registry) EnvelopeDef() Envelope {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return registry.envelope
}

// Lang configures request language resolution.
func (registry *Registry) Lang(config Lang) {
	registry.mu.Lock()
	registry.lang = Lang{
		Default: config.Default,
		Sources: append([]LangSource(nil), config.Sources...),
	}
	registry.runtime = nil
	registry.mu.Unlock()
}

// LangConfig returns request language resolution config.
func (registry *Registry) LangConfig() Lang {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return Lang{
		Default: registry.lang.Default,
		Sources: append([]LangSource(nil), registry.lang.Sources...),
	}
}

// Translator sets the app-level translator.
func (registry *Registry) Translator(translator Translator) {
	registry.mu.Lock()
	registry.translator = translator
	registry.runtime = nil
	registry.mu.Unlock()
}

// TranslatorDef returns the app-level translator.
func (registry *Registry) TranslatorDef() Translator {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return registry.translator
}

// Config sets the app config store.
func (registry *Registry) Config(store *config.Store) {
	registry.mu.Lock()
	registry.config = store
	registry.runtime = nil
	registry.mu.Unlock()
}

// ConfigDef returns the app config store.
func (registry *Registry) ConfigDef() *config.Store {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return registry.config
}

// Service binds an app-scoped service into route contexts.
func (registry *Registry) Service(service any, name ...string) {
	if registry == nil || service == nil {
		return
	}
	registry.mu.Lock()
	if registry.services == nil {
		registry.services = make(map[string]any)
	}
	registry.services[typeKey(service)] = service
	for _, item := range name {
		if item != "" {
			registry.services[item] = service
		}
	}
	registry.runtime = nil
	registry.mu.Unlock()
}

// Services returns a copy of app-scoped route services.
func (registry *Registry) Services() map[string]any {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	items := make(map[string]any, len(registry.services))
	for key, value := range registry.services {
		items[key] = value
	}
	return items
}

// Register registers or replaces a route by method and path.
func (registry *Registry) Register(route *Route) *Route {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	key := route.Method + " " + route.Path
	if index, ok := registry.index[key]; ok {
		registry.routes[index] = route
		registry.runtime = nil
		return route
	}
	registry.index[key] = len(registry.routes)
	registry.routes = append(registry.routes, route)
	registry.runtime = nil
	return route
}

// Routes returns route snapshots in registration order.
func (registry *Registry) Routes() []*Route {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return append([]*Route(nil), registry.routes...)
}

// Build builds the router handler.
func (registry *Registry) Build() (http.Handler, error) {
	registry.mu.Lock()
	if registry.runtime != nil {
		runtime := registry.runtime
		registry.mu.Unlock()
		return runtime, nil
	}
	registry.mu.Unlock()

	runtime, err := buildRuntime(registry)
	if err != nil {
		return nil, err
	}

	registry.mu.Lock()
	registry.runtime = runtime
	registry.mu.Unlock()
	return runtime, nil
}

// Handler returns the built HTTP handler.
func (registry *Registry) Handler() http.Handler {
	runtime, err := registry.Build()
	if err != nil {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
		})
	}
	return runtime
}
