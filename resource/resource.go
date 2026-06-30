package resource

import (
	"strings"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
)

// Resource groups route actions under a resource path and metadata.
type Resource struct {
	group       *route.Group
	path        string
	name        string
	summary     string
	description string
	tags        []string
	security    []route.Security
	docNames    []string
	skipDoc     bool
	middleware  []route.Middleware
	meta        core.Map
	flags       core.Map
	routes      map[string]*route.Route
}

// New creates a resource under a route group.
func New(group *route.Group, path string) *Resource {
	return &Resource{
		group:  group,
		path:   cleanPath(path),
		meta:   make(core.Map),
		flags:  make(core.Map),
		routes: make(map[string]*route.Route),
	}
}

// RouteGroup returns the underlying route group.
func (res *Resource) RouteGroup() *route.Group { return res.group }

// Path returns the resource base path.
func (res *Resource) Path() string { return res.path }

// Name sets the resource name prefix.
func (res *Resource) Name(name string) *Resource {
	res.name = strings.Trim(name, ".")
	return res
}

// NameValue returns the resource name.
func (res *Resource) NameValue() string { return res.name }

// Summary sets the resource summary.
func (res *Resource) Summary(value string) *Resource {
	res.summary = value
	return res
}

// SummaryValue returns the resource summary.
func (res *Resource) SummaryValue() string { return res.summary }

// Description sets the resource description.
func (res *Resource) Description(value string) *Resource {
	res.description = value
	return res
}

// Tags sets default route tags.
func (res *Resource) Tags(values ...string) *Resource {
	res.tags = append([]string(nil), values...)
	return res
}

// Security sets default route security.
func (res *Resource) Security(values ...route.Security) *Resource {
	res.security = append([]route.Security(nil), values...)
	return res
}

// Doc sets OpenAPI document domains for this resource.
func (res *Resource) Doc(names ...string) *Resource {
	res.docNames = append([]string(nil), names...)
	res.skipDoc = false
	return res
}

// SkipDoc excludes this resource from OpenAPI documents.
func (res *Resource) SkipDoc() *Resource {
	res.docNames = nil
	res.skipDoc = true
	return res
}

// Use appends default route middleware.
func (res *Resource) Use(middleware ...route.Middleware) *Resource {
	res.middleware = append(res.middleware, middleware...)
	return res
}

// Meta sets default route metadata.
func (res *Resource) Meta(key string, value any) *Resource {
	res.meta[key] = value
	return res
}

// SkipAuth marks resource routes as skipping auth middleware.
func (res *Resource) SkipAuth() *Resource {
	res.flags["skip_auth"] = true
	return res
}

// OptionalAuth marks resource routes as optional auth.
func (res *Resource) OptionalAuth() *Resource {
	res.flags["optional_auth"] = true
	return res
}

// SkipPermission marks resource routes as skipping permission middleware.
func (res *Resource) SkipPermission() *Resource {
	res.flags["skip_permission"] = true
	return res
}

// Route returns a registered action route.
func (res *Resource) Route(name string) *route.Route {
	return res.routes[name]
}

func (res *Resource) route(method string, action string, path string, handler route.Handler) *route.Route {
	fullPath := joinPath(res.path, path)
	item := res.group.Handle(method, fullPath, handler)
	res.apply(action, item)
	return item
}

func (res *Resource) apply(action string, item *route.Route) {
	name := joinName(res.name, action)
	if name != "" {
		item.Name(name)
	}
	if res.summary != "" {
		item.Summary(defaultSummary(res.summary, action))
	}
	if res.description != "" {
		item.Description(res.description)
	}
	if len(res.tags) > 0 {
		item.Tags(res.tags...)
	}
	if len(res.security) > 0 {
		values := make([]string, 0, len(res.security))
		for _, security := range res.security {
			values = append(values, string(security))
		}
		item.Security(values...)
	}
	if res.skipDoc {
		item.SkipDoc()
	} else if len(res.docNames) > 0 {
		item.Doc(res.docNames...)
	}
	if len(res.middleware) > 0 {
		item.Use(res.middleware...)
	}
	for key, value := range res.meta {
		item.Meta(key, value)
	}
	for key, value := range res.flags {
		item.Meta(key, value)
	}
	item.Meta("resource", res.name)
	item.Meta("action", action)
	res.routes[action] = item
}

func defaultSummary(summary string, action string) string {
	switch action {
	case "list":
		return summary + "列表"
	case "show":
		return summary + "详情"
	case "create":
		return "创建" + summary
	case "edit":
		return "编辑" + summary
	case "store":
		return "保存" + summary
	case "delete":
		return "删除" + summary
	case "batch":
		return "批量操作" + summary
	case "import":
		return "导入" + summary
	case "export":
		return "导出" + summary
	case "restore":
		return "恢复" + summary
	case "destroy":
		return "彻底删除" + summary
	default:
		return summary
	}
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
	return prefix + "." + name
}
