package route

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/runtime"
)

type routeDomainModule struct{ runtime.ModuleBase }

func (routeDomainModule) Name() string { return "admin.route" }
func (routeDomainModule) Init(ctx context.Context, app provider.Context) error {
	routes := provider.MustInvoke[*Registry](app)
	routes.Domain("admin", "/admin", func(admin *Group) {
		admin.Group("/settings").Name("settings")
	})
	return nil
}

type routeDomainFeatureModule struct{ runtime.ModuleBase }

func (routeDomainFeatureModule) Name() string { return "admin.feature" }
func (routeDomainFeatureModule) Register(ctx context.Context, app provider.Context) error {
	routes := provider.MustInvoke[*Registry](app)
	routes.MountDomain("admin", func(group *Group) {
		group.Get("/feature", func(ctx *Context) error { return ctx.Text("feature") }).Name("feature.index")
	})
	routes.MountGroup("admin.settings", func(group *Group) {
		group.Get("/profile", func(ctx *Context) error { return ctx.Text("profile") }).Name("profile")
	})
	return nil
}

func TestProviderMountsRouteDomainAcrossModules(t *testing.T) {
	app := runtime.New()
	app.Install(Provider())
	app.Module(routeDomainFeatureModule{}, routeDomainModule{})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	routes := provider.MustInvoke[*Registry](app)
	if routes.GetDomain("admin") == nil {
		t.Fatal("missing admin domain")
	}
	if routes.GetGroup("admin.settings") == nil {
		t.Fatal("missing admin.settings group")
	}
	response := httptest.NewRecorder()
	routes.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/feature", nil))
	if response.Body.String() != "feature" {
		t.Fatalf("body = %q", response.Body.String())
	}
	response = httptest.NewRecorder()
	routes.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/settings/profile", nil))
	if response.Body.String() != "profile" {
		t.Fatalf("nested body = %q", response.Body.String())
	}
	found := false
	for _, item := range routes.Routes() {
		if item.RouteName == "admin.settings.profile" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing nested route name: %#v", routes.Routes())
	}
}

func TestProviderRouteDomainPrefixesRouteNames(t *testing.T) {
	app := runtime.New()
	registry := New()
	app.Install(Provider(UseRegistry(registry)))
	registry.Domain("api", "/api", func(api *Group) {
		api.Get("/users", func(ctx *Context) error { return ctx.Text("api") }).Name("user.list")
	})
	registry.Domain("admin", "/admin", func(admin *Group) {
		admin.Get("/users", func(ctx *Context) error { return ctx.Text("admin") }).Name("user.list")
	})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	names := map[string]bool{}
	for _, item := range registry.Routes() {
		names[item.RouteName] = true
	}
	if !names["api.user.list"] || !names["admin.user.list"] {
		t.Fatalf("route names = %#v", names)
	}
}

type missingRouteDomainModule struct{ runtime.ModuleBase }

func (missingRouteDomainModule) Name() string { return "missing.domain.feature" }
func (missingRouteDomainModule) Register(ctx context.Context, app provider.Context) error {
	routes := provider.MustInvoke[*Registry](app)
	routes.MountDomain("missing", func(group *Group) {
		group.Get("/feature", func(ctx *Context) error { return ctx.Text("feature") })
	})
	return nil
}

func TestProviderMountMissingRouteDomainFailsFreeze(t *testing.T) {
	app := runtime.New()
	app.Install(Provider())
	app.Module(missingRouteDomainModule{})
	if err := app.Freeze(context.Background()); err == nil || !strings.Contains(err.Error(), "route domain missing is not registered") {
		t.Fatalf("freeze err = %v", err)
	}
}

type missingRouteGroupModule struct{ runtime.ModuleBase }

func (missingRouteGroupModule) Name() string { return "missing.group.feature" }
func (missingRouteGroupModule) Register(ctx context.Context, app provider.Context) error {
	routes := provider.MustInvoke[*Registry](app)
	routes.MountGroup("admin.missing", func(group *Group) {
		group.Get("/feature", func(ctx *Context) error { return ctx.Text("feature") })
	})
	return nil
}

func TestProviderMountMissingRouteGroupFailsFreeze(t *testing.T) {
	app := runtime.New()
	app.Install(Provider())
	app.Module(missingRouteGroupModule{})
	if err := app.Freeze(context.Background()); err == nil || !strings.Contains(err.Error(), "route group admin.missing is not registered") {
		t.Fatalf("freeze err = %v", err)
	}
}
