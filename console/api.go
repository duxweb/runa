package console

import (
	"net/http"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
)

// RouteInfo is a serializable route snapshot.
type RouteInfo struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Name        string   `json:"name,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Security    []string `json:"security,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
	Status      int      `json:"status,omitempty"`
	Middlewares int      `json:"middlewares"`
	DocNames    []string `json:"docs,omitempty"`
	SkipDoc     bool     `json:"skip_doc,omitempty"`
	Meta        core.Map `json:"meta,omitempty"`
}

// Routes returns serializable route snapshots.
func Routes(app AppContext) []RouteInfo {
	if app == nil {
		return nil
	}
	items := appRoutes(app)
	routes := make([]RouteInfo, 0, len(items))
	for _, item := range items {
		routes = append(routes, routeInfo(item))
	}
	return routes
}

// PublicRoutes returns route snapshots intended for the console routes page.
func PublicRoutes(app AppContext) []RouteInfo {
	items := Routes(app)
	routes := make([]RouteInfo, 0, len(items))
	for _, item := range items {
		if item.SkipDoc {
			continue
		}
		routes = append(routes, item)
	}
	return routes
}

func routeInfo(item *route.Route) RouteInfo {
	if item == nil {
		return RouteInfo{}
	}
	status := item.SuccessStatus
	if status == 0 {
		status = http.StatusOK
	}
	security := make([]string, 0, len(item.SecurityList))
	for _, item := range item.SecurityList {
		security = append(security, string(item))
	}
	return RouteInfo{
		Method:      item.Method,
		Path:        item.Path,
		Name:        item.RouteName,
		Summary:     item.SummaryText,
		Description: item.DescriptionText,
		Tags:        append([]string(nil), item.TagList...),
		Security:    security,
		Deprecated:  item.DeprecatedFlag,
		Status:      status,
		Middlewares: len(item.Middlewares),
		DocNames:    append([]string(nil), item.DocNames...),
		SkipDoc:     item.SkipDocument,
		Meta:        core.CloneMap(item.MetaData),
	}
}
