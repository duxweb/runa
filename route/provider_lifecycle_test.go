package route

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runacommand "github.com/duxweb/runa/command"
	"github.com/duxweb/runa/host"
	"github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/runtime"
	"github.com/samber/do/v2"
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

type routeProviderContext struct {
	app      any
	injector do.Injector
	hosts    []host.Unit
	commands []runacommand.Command
}

func (ctx *routeProviderContext) App() any              { return ctx.app }
func (ctx *routeProviderContext) Injector() do.Injector { return ctx.injector }
func (ctx *routeProviderContext) RegisterCommand(commands ...runacommand.Command) error {
	ctx.commands = append(ctx.commands, commands...)
	return nil
}
func (ctx *routeProviderContext) RegisterService(...any) error { return nil }
func (ctx *routeProviderContext) RegisterModule(...any) error  { return nil }
func (ctx *routeProviderContext) RegisterHost(units ...host.Unit) error {
	ctx.hosts = append(ctx.hosts, units...)
	return nil
}
func (ctx *routeProviderContext) RegisterRouteService(...any) error { return nil }

type routeProviderApp struct {
	writer *bytes.Buffer
	env    string
}

func (app routeProviderApp) Writer() io.Writer { return app.writer }
func (app routeProviderApp) Env() string       { return app.env }

func TestProviderRegistersStartupBannerHost(t *testing.T) {
	var out bytes.Buffer
	ctx := &routeProviderContext{app: routeProviderApp{writer: &out, env: "testing"}, injector: do.New()}
	item := Provider(Addr(":0"))
	if err := item.Init(context.Background(), ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := item.Register(ctx); err != nil {
		t.Fatalf("register: %v", err)
	}
	if len(ctx.hosts) != 1 {
		t.Fatalf("hosts = %#v", ctx.hosts)
	}
	if err := ctx.hosts[0].Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer ctx.hosts[0].Stop(context.Background())
	if !strings.Contains(out.String(), "Runa HTTP") || !strings.Contains(out.String(), "Env") || !strings.Contains(out.String(), "testing") {
		t.Fatalf("banner output = %q", out.String())
	}
}

func TestProviderCanDisableStartupBanner(t *testing.T) {
	var out bytes.Buffer
	ctx := &routeProviderContext{app: routeProviderApp{writer: &out, env: "testing"}, injector: do.New()}
	item := Provider(Addr(":0"), Banner(false))
	if err := item.Init(context.Background(), ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := item.Register(ctx); err != nil {
		t.Fatalf("register: %v", err)
	}
	if len(ctx.hosts) != 1 {
		t.Fatalf("hosts = %#v", ctx.hosts)
	}
	if err := ctx.hosts[0].Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer ctx.hosts[0].Stop(context.Background())
	if out.Len() != 0 {
		t.Fatalf("banner output = %q", out.String())
	}
}
