package route

import "github.com/duxweb/runa/core"

// Route stores route metadata and handler.
type Route struct {
	Method          string
	Path            string
	FullPath        string
	Handler         Handler
	Middlewares     []Middleware
	MetaData        core.Map
	RouteName       string
	SummaryText     string
	DescriptionText string
	TagList         []string
	SecurityList    []Security
	DeprecatedFlag  bool
	DocNames        []string
	SkipDocument    bool
	SchemaDef       *SchemaInfo
	Errors          ErrorPipeline
	EnvelopeDef     Envelope
	RawResponse     bool
	SuccessStatus   int
	ViewDomainName  string
	namePrefix      string
}

// Name sets the route name.
func (route *Route) Name(name string) *Route {
	route.RouteName = joinName(route.namePrefix, name)
	return route
}

// Use appends route middlewares.
func (route *Route) Use(middlewares ...Middleware) *Route {
	route.Middlewares = append(route.Middlewares, middlewares...)
	return route
}

// Meta sets route metadata.
func (route *Route) Meta(key string, value any) *Route {
	if route.MetaData == nil {
		route.MetaData = make(core.Map)
	}
	route.MetaData[key] = value
	return route
}

// MetaAs reads route metadata cast to T.
func (route *Route) MetaAs[T any](key string, fallback ...T) T {
	if route == nil {
		return core.Cast[T](nil, fallback...)
	}
	if route.MetaData == nil {
		return core.Cast[T](nil, fallback...)
	}
	return core.Cast[T](route.MetaData[key], fallback...)
}

// RouteID returns the full route name used by permissions.
func (route *Route) RouteID() string { return route.RouteName }

// SkipAuth marks this route as not requiring authentication.
func (route *Route) SkipAuth() *Route {
	return route.Meta("auth", false)
}

// OptionalAuth marks this route as optionally authenticated.
func (route *Route) OptionalAuth() *Route {
	return route.Meta("auth", "optional")
}

// SkipPermission marks this route as not requiring permission checks.
func (route *Route) SkipPermission() *Route {
	return route.Meta("can", false)
}

// Summary sets route OpenAPI summary metadata.
func (route *Route) Summary(summary string) *Route {
	route.SummaryText = summary
	return route
}

// Description sets route OpenAPI description metadata.
func (route *Route) Description(description string) *Route {
	route.DescriptionText = description
	return route
}

// Tags sets route OpenAPI tags metadata.
func (route *Route) Tags(tags ...string) *Route {
	route.TagList = append([]string(nil), tags...)
	return route
}

// Security sets route OpenAPI security metadata.
func (route *Route) Security(security ...string) *Route {
	route.SecurityList = append([]Security(nil), security...)
	return route
}

// Doc sets OpenAPI document domains for this route.
func (route *Route) Doc(names ...string) *Route {
	route.DocNames = append([]string(nil), names...)
	route.SkipDocument = false
	return route
}

// SkipDoc excludes this route from OpenAPI documents.
func (route *Route) SkipDoc() *Route {
	route.DocNames = nil
	route.SkipDocument = true
	return route
}

// Deprecated marks the route as deprecated.
func (route *Route) Deprecated(value ...bool) *Route {
	route.DeprecatedFlag = true
	if len(value) > 0 {
		route.DeprecatedFlag = value[0]
	}
	return route
}

// OnError sets the route error handler.
func (route *Route) OnError(handler ErrorHandler) *Route {
	route.Errors.OnError = handler
	return route
}

// Error sets the route error renderer.
func (route *Route) Error(renderer ErrorRenderer) *Route {
	route.Errors.Renderer = renderer
	return route
}

// Envelope sets the route response envelope.
func (route *Route) Envelope(envelope Envelope) *Route {
	route.EnvelopeDef = envelope
	route.RawResponse = false
	return route
}

// Raw disables response envelope for the route.
func (route *Route) Raw() *Route {
	route.RawResponse = true
	return route
}

// Status sets the default success status for typed output.
func (route *Route) Status(code int) *Route {
	route.SuccessStatus = code
	return route
}

// ViewDomain sets the default view domain for this route.
func (route *Route) ViewDomain(name string) *Route {
	route.ViewDomainName = name
	return route
}

// Schema sets OpenAPI-neutral route schema metadata.
func (route *Route) Schema(schema SchemaInfo) *Route {
	route.SchemaDef = &schema
	return route
}
