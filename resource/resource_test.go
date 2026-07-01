package resource

import (
	"net/http"
	"testing"

	"github.com/duxweb/runa/route"
)

type resourceInput struct{}
type resourceOutput struct {
	OK bool `json:"ok"`
}

func TestResourceRegistersNamedActions(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "/admin").Name("admin")
	users := New(group, "/users").
		Name("system.user").
		Summary("用户").
		Tags("User").
		Doc("admin").
		Meta("permission", "system.user")

	users.List[resourceInput, resourceOutput](func(ctx *route.Context, input *resourceInput) (*resourceOutput, error) {
		return &resourceOutput{OK: true}, nil
	})
	users.Post[resourceInput, resourceOutput]("import", "/import", func(ctx *route.Context, input *resourceInput) (*resourceOutput, error) {
		return &resourceOutput{OK: true}, nil
	}).Status(http.StatusAccepted)

	routes := registry.Routes()
	if len(routes) != 2 {
		t.Fatalf("routes len = %d", len(routes))
	}
	if routes[0].Path != "/admin/users" || routes[0].RouteName != "admin.system.user.list" {
		t.Fatalf("route[0] = %#v", routes[0])
	}
	if routes[0].SummaryText != "用户列表" {
		t.Fatalf("summary = %q", routes[0].SummaryText)
	}
	if len(routes[0].DocNames) != 1 || routes[0].DocNames[0] != "admin" {
		t.Fatalf("doc names = %#v", routes[0].DocNames)
	}
	if routes[0].MetaAs[string]("permission") != "system.user" {
		t.Fatalf("meta = %#v", routes[0].MetaData)
	}
	if routes[1].Path != "/admin/users/import" || routes[1].RouteName != "admin.system.user.import" {
		t.Fatalf("route[1] = %#v", routes[1])
	}
	if routes[1].SuccessStatus != http.StatusAccepted {
		t.Fatalf("status = %d", routes[1].SuccessStatus)
	}
	if routes[0].SchemaDef == nil || routes[0].SchemaDef.Response.Type != "object" {
		t.Fatalf("schema = %#v", routes[0].SchemaDef)
	}
	if users.Route("list").MetaAs[string]("action") != "list" {
		t.Fatalf("route meta = %#v", users.Route("list").MetaData)
	}
}

func TestResourceSkipDoc(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "/admin").Doc("admin")
	New(group, "/internal").Name("internal").SkipDoc().
		List[resourceInput, resourceOutput](func(ctx *route.Context, input *resourceInput) (*resourceOutput, error) {
		return &resourceOutput{OK: true}, nil
	})
	routes := registry.Routes()
	if len(routes) != 1 || !routes[0].SkipDocument {
		t.Fatalf("routes = %#v", routes)
	}
}
