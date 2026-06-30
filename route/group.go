package route

import "strings"

// Group is a route registration scope.
type Group struct {
	registry      *Registry
	prefix        string
	middlewares   []Middleware
	errors        ErrorPipeline
	envelope      Envelope
	raw           bool
	name          string
	docNames      []string
	skipDoc       bool
	summary       string
	description   string
	tags          []string
	security      []Security
	viewDomain    string
	meta          map[string]any
	deprecated    bool
	deprecatedSet bool
}

// NewGroup creates a route group.
func NewGroup(registry *Registry, prefix string, middlewares ...Middleware) *Group {
	return &Group{
		registry:    registry,
		prefix:      cleanPath(prefix),
		middlewares: append([]Middleware(nil), middlewares...),
	}
}

// RouteGroup returns the group itself.
func (group *Group) RouteGroup() *Group { return group }

// Registry returns the route registry that owns the group.
func (group *Group) Registry() *Registry { return group.registry }

// Use appends group middlewares.
func (group *Group) Use(middlewares ...Middleware) *Group {
	group.middlewares = append(group.middlewares, middlewares...)
	return group
}

// Group creates a nested group.
func (group *Group) Group(prefix string, fn ...func(*Group)) *Group {
	child := &Group{
		registry:      group.registry,
		prefix:        joinPath(group.prefix, prefix),
		middlewares:   append([]Middleware(nil), group.middlewares...),
		errors:        group.errors,
		envelope:      group.envelope,
		raw:           group.raw,
		name:          group.name,
		docNames:      append([]string(nil), group.docNames...),
		skipDoc:       group.skipDoc,
		summary:       group.summary,
		description:   group.description,
		tags:          append([]string(nil), group.tags...),
		security:      append([]Security(nil), group.security...),
		viewDomain:    group.viewDomain,
		meta:          cloneMeta(group.meta),
		deprecated:    group.deprecated,
		deprecatedSet: group.deprecatedSet,
	}
	for _, callback := range fn {
		if callback != nil {
			callback(child)
		}
	}
	return child
}

// Name sets the group name prefix metadata.
func (group *Group) Name(name string) *Group {
	group.name = joinName(group.name, name)
	if group.registry != nil {
		group.registry.RegisterGroup(group.name, group)
	}
	return group
}

// Doc sets OpenAPI document domains for the group.
func (group *Group) Doc(names ...string) *Group {
	group.docNames = append([]string(nil), names...)
	group.skipDoc = false
	return group
}

// SkipDoc excludes the group and children from OpenAPI documents.
func (group *Group) SkipDoc() *Group {
	group.docNames = nil
	group.skipDoc = true
	return group
}

// Summary sets group OpenAPI summary metadata.
func (group *Group) Summary(summary string) *Group {
	group.summary = summary
	return group
}

// Description sets group OpenAPI description metadata.
func (group *Group) Description(description string) *Group {
	group.description = description
	return group
}

// Tags sets group OpenAPI tags metadata.
func (group *Group) Tags(tags ...string) *Group {
	group.tags = append([]string(nil), tags...)
	return group
}

// Security sets group OpenAPI security metadata.
func (group *Group) Security(security ...Security) *Group {
	group.security = append([]Security(nil), security...)
	return group
}

// Meta sets group metadata for child routes.
func (group *Group) Meta(key string, value any) *Group {
	if group.meta == nil {
		group.meta = make(map[string]any)
	}
	group.meta[key] = value
	return group
}

// SkipAuth marks group routes as not requiring authentication.
func (group *Group) SkipAuth() *Group {
	return group.Meta("auth", false)
}

// OptionalAuth marks group routes as optionally authenticated.
func (group *Group) OptionalAuth() *Group {
	return group.Meta("auth", "optional")
}

// SkipPermission marks group routes as not requiring permission checks.
func (group *Group) SkipPermission() *Group {
	return group.Meta("can", false)
}

// Deprecated marks group routes as deprecated.
func (group *Group) Deprecated(value ...bool) *Group {
	group.deprecated = true
	if len(value) > 0 {
		group.deprecated = value[0]
	}
	group.deprecatedSet = true
	return group
}

// OnError sets the group error handler.
func (group *Group) OnError(handler ErrorHandler) *Group {
	group.errors.OnError = handler
	return group
}

// Error sets the group error renderer.
func (group *Group) Error(renderer ErrorRenderer) *Group {
	group.errors.Renderer = renderer
	return group
}

// Envelope sets the group response envelope.
func (group *Group) Envelope(envelope Envelope) *Group {
	group.envelope = envelope
	group.raw = false
	return group
}

// Raw disables response envelope for the group.
func (group *Group) Raw() *Group {
	group.raw = true
	return group
}

// ViewDomain sets the default view domain for routes registered in this group.
func (group *Group) ViewDomain(name string) *Group {
	group.viewDomain = name
	return group
}

// Mount mounts route registrations into the group.
func (group *Group) Mount(register func(*Group)) *Group {
	if register != nil {
		register(group)
	}
	return group
}

// Get registers a GET route.
func (group *Group) Get(path string, handler Handler) *Route {
	return group.Handle("GET", path, handler)
}

// Post registers a POST route.
func (group *Group) Post(path string, handler Handler) *Route {
	return group.Handle("POST", path, handler)
}

// Put registers a PUT route.
func (group *Group) Put(path string, handler Handler) *Route {
	return group.Handle("PUT", path, handler)
}

// Patch registers a PATCH route.
func (group *Group) Patch(path string, handler Handler) *Route {
	return group.Handle("PATCH", path, handler)
}

// Delete registers a DELETE route.
func (group *Group) Delete(path string, handler Handler) *Route {
	return group.Handle("DELETE", path, handler)
}

// Options registers an OPTIONS route.
func (group *Group) Options(path string, handler Handler) *Route {
	return group.Handle("OPTIONS", path, handler)
}

// Head registers a HEAD route.
func (group *Group) Head(path string, handler Handler) *Route {
	return group.Handle("HEAD", path, handler)
}

// Any registers a route for common HTTP methods.
func (group *Group) Any(path string, handler Handler) *Route {
	return group.Handle("ANY", path, handler)
}

// Handle registers a route by method.
func (group *Group) Handle(method string, path string, handler Handler) *Route {
	fullPath := joinPath(group.prefix, path)
	route := &Route{
		Method:          strings.ToUpper(method),
		Path:            fullPath,
		FullPath:        fullPath,
		Handler:         handler,
		Middlewares:     append([]Middleware(nil), group.middlewares...),
		Errors:          group.errors,
		EnvelopeDef:     group.envelope,
		RawResponse:     group.raw,
		ViewDomainName:  group.viewDomain,
		DocNames:        append([]string(nil), group.docNames...),
		SkipDocument:    group.skipDoc,
		SummaryText:     group.summary,
		DescriptionText: group.description,
		TagList:         append([]string(nil), group.tags...),
		SecurityList:    append([]Security(nil), group.security...),
		DeprecatedFlag:  group.deprecated,
		MetaData:        cloneMeta(group.meta),
		namePrefix:      group.name,
	}
	if group.deprecatedSet {
		route.DeprecatedFlag = group.deprecated
	}
	return group.registry.Register(route)
}

func cleanPath(path string) string {
	if path == "" || path == "/" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(path, "/")
}

func joinPath(prefix string, path string) string {
	prefix = cleanPath(prefix)
	if path == "" || path == "/" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if prefix == "" {
		return path
	}
	return prefix + path
}

func joinName(prefix string, name string) string {
	prefix = strings.Trim(prefix, ".")
	name = strings.Trim(name, ".")
	if prefix == "" {
		return name
	}
	if name == "" {
		return prefix
	}
	if strings.HasPrefix(name, prefix+".") || name == prefix {
		return name
	}
	return prefix + "." + name
}

func cloneMeta(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	cloned := make(map[string]any, len(meta))
	for key, value := range meta {
		cloned[key] = value
	}
	return cloned
}
