package console

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/auth"
	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/errs"
	"github.com/duxweb/runa/jsonrpc"
	"github.com/duxweb/runa/lock"
	"github.com/duxweb/runa/message"
	"github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/rate"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/runtime"
	"github.com/duxweb/runa/session"
	"github.com/duxweb/runa/ws"
)

func TestMountConsoleRoutes(t *testing.T) {
	app, routes := newConsoleApp()
	Mount(routes.Group("/__runa"), app, Config{Title: "Console"})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := request(app, "/__runa/overview/api/components")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status"`) || !strings.Contains(response.Body.String(), `"queue_states"`) {
		t.Fatalf("components = %d %q", response.Code, response.Body.String())
	}
}

func TestOverviewInfrastructureSummaryShape(t *testing.T) {
	app, routes := newConsoleApp()
	app.Install(cache.Provider())
	Mount(routes.Group("/__runa"), app, Config{Title: "Console"})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := request(app, "/__runa/overview/api/components")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	var components []ComponentInfo
	if err := json.Unmarshal(response.Body.Bytes(), &components); err != nil {
		t.Fatalf("decode components: %v body=%q", err, response.Body.String())
	}
	for _, component := range components {
		if component.ID != "infrastructure" {
			continue
		}
		raw, err := json.Marshal(component.Data)
		if err != nil {
			t.Fatalf("encode infrastructure: %v", err)
		}
		var rows []Summary
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatalf("decode infrastructure: %v data=%s", err, raw)
		}
		for _, row := range rows {
			if row.Module == "" {
				t.Fatalf("empty module: %#v", row)
			}
			if row.Module == "Cache" {
				if row.Summary != "memory" || row.Default != "memory" {
					t.Fatalf("cache summary = %#v", row)
				}
				return
			}
		}
		t.Fatalf("cache summary missing: %#v", rows)
	}
	t.Fatalf("infrastructure component missing: %#v", components)
}

func TestMountConsoleRouteSnapshots(t *testing.T) {
	app, routes := newConsoleApp()
	routes.Get("/ping", func(ctx *route.Context) error {
		return ctx.Text("pong")
	}).Name("ping").Summary("Ping").Meta("scope", "test")
	Mount(routes.Group("/__runa"), app, Config{Title: "Console"})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := request(app, "/__runa/routes/api/components")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	var components []ComponentInfo
	if err := json.Unmarshal(response.Body.Bytes(), &components); err != nil {
		t.Fatalf("decode components: %v body=%q", err, response.Body.String())
	}
	var routeRows struct {
		Rows []map[string]any `json:"rows"`
	}
	for _, component := range components {
		if component.ID != "routes" {
			continue
		}
		raw, err := json.Marshal(component.Data)
		if err != nil {
			t.Fatalf("encode routes component: %v", err)
		}
		if err := json.Unmarshal(raw, &routeRows); err != nil {
			t.Fatalf("decode routes: %v data=%s", err, raw)
		}
	}
	for _, item := range routeRows.Rows {
		if item["skip_doc"] == true {
			t.Fatalf("routes panel should hide skip-doc route: %#v", item)
		}
		if item["name"] == "ping" {
			if item["method"] != http.MethodGet || item["path"] != "/ping" || item["summary"] != "Ping" {
				t.Fatalf("route = %#v", item)
			}
			return
		}
	}
	t.Fatalf("ping route missing: %#v", routeRows)
}

func TestProviderAutoMount(t *testing.T) {
	app, _ := newConsoleApp()
	app.Install(Provider(MountAt("/__runa")))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := request(app, "/__runa")
	if response.Code != http.StatusFound || response.Header().Get("Location") != "/__runa/overview" {
		t.Fatalf("redirect = %d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	page := request(app, "/__runa/overview")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Runa Console") || !strings.Contains(page.Body.String(), "Alpine") {
		t.Fatalf("console = %d %q", page.Code, page.Body.String())
	}
}

func TestProviderReadsConfig(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "console.toml", `mount = "/ops"
title = "Ops Console"
`)
	app, _ := newConsoleApp(runa.BasePath(root))
	app.Install(Provider())
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	page := request(app, "/ops/overview")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Ops Console") {
		t.Fatalf("console = %d %q", page.Code, page.Body.String())
	}
}

func writeConfig(t *testing.T, root string, name string, body string) {
	t.Helper()
	dir := filepath.Join(root, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestProviderMountsPanels(t *testing.T) {
	app, _ := newConsoleApp()
	app.Install(Provider(MountAt("/__runa"), Panels(PanelFunc{
		Name:  "test",
		Title: "Test Panel",
		Icon:  "T",
		Order: 20,
		Mount: func(group *route.Group) {
			group.Get("/ping", func(ctx *route.Context) error { return ctx.Text("pong") })
		},
	})))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	panels := request(app, "/__runa/api/panels")
	if panels.Code != http.StatusOK || !strings.Contains(panels.Body.String(), `"test"`) || !strings.Contains(panels.Body.String(), `"Test Panel"`) {
		t.Fatalf("panels = %d %q", panels.Code, panels.Body.String())
	}
	response := request(app, "/__runa/test/ping")
	if response.Code != http.StatusOK || response.Body.String() != "pong" {
		t.Fatalf("panel = %d %q", response.Code, response.Body.String())
	}
}

func TestProviderMountsComponentPanel(t *testing.T) {
	app, _ := newConsoleApp()
	app.Install(Provider(MountAt("/__runa"), Panels(ComponentPanel{
		Name:  "sample",
		Title: "Sample",
		Icon:  "S",
		Order: 10,
		Components: []Component{
			{Name: "total", Label: "Total", Type: ComponentMetric, Resolve: func(context.Context, AppContext) (any, error) { return 3, nil }},
			{Name: "health", Label: "Health", Type: ComponentStatus, Resolve: func(context.Context, AppContext) (any, error) { return core.Map{"status": "ok"}, nil }},
			{Name: "items", Label: "Items", Type: ComponentTable, Resolve: func(context.Context, AppContext) (any, error) {
				return []core.Map{{"name": "one"}}, nil
			}},
			{Name: "trend", Label: "Trend", Type: ComponentLine, Resolve: func(context.Context, AppContext) (any, error) {
				return []core.Map{{"label": "a", "value": 1}, {"label": "b", "value": 2}}, nil
			}},
			{Name: "bars", Label: "Bars", Type: ComponentBar, Resolve: func(context.Context, AppContext) (any, error) {
				return []core.Map{{"label": "a", "value": 1}, {"label": "b", "value": 2}}, nil
			}},
		},
	})))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	page := request(app, "/__runa/sample")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Sample") || !strings.Contains(page.Body.String(), "componentPanel") {
		t.Fatalf("page = %d %q", page.Code, page.Body.String())
	}
	response := request(app, "/__runa/sample/api/components")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"total"`) || !strings.Contains(response.Body.String(), `"items"`) {
		t.Fatalf("components = %d %q", response.Code, response.Body.String())
	}
}

func TestExternalProviderRegistersConsolePanel(t *testing.T) {
	app, _ := newConsoleApp()
	app.Install(Provider(MountAt("/__runa")))
	app.Install(panelProvider{})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	panels := request(app, "/__runa/api/panels")
	if panels.Code != http.StatusOK || !strings.Contains(panels.Body.String(), `"external"`) {
		t.Fatalf("panels = %d %q", panels.Code, panels.Body.String())
	}
	response := request(app, "/__runa/external/ping")
	if response.Code != http.StatusOK || response.Body.String() != "pong" {
		t.Fatalf("external panel = %d %q", response.Code, response.Body.String())
	}
}

func TestConsoleAuthOptionProtectsRoutes(t *testing.T) {
	app, _ := newConsoleApp()
	app.Install(auth.Provider())
	app.Install(authRegisterProvider{})
	app.Install(Provider(MountAt("/__runa"), Auth("console")))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if response := request(app, "/__runa/overview/api/components"); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized = %d %q", response.Code, response.Body.String())
	}
	request := httptest.NewRequest(http.MethodGet, "/__runa/overview/api/components", nil)
	request.Header.Set("X-API-Key", "secret")
	response := httptest.NewRecorder()
	routesOf(app).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"routes"`) {
		t.Fatalf("authorized = %d %q", response.Code, response.Body.String())
	}
}

func TestConsoleMonitorCapturesHTTPAccessAndErrors(t *testing.T) {
	app, routes := newConsoleApp()
	routes.Get("/ok", func(ctx *route.Context) error {
		return ctx.Text("ok")
	}).Name("ok")
	routes.Get("/boom", func(ctx *route.Context) error {
		return errs.Wrap(ctx.Error(http.StatusInternalServerError, "boom"))
	}).Name("boom")
	app.Install(Provider(MountAt("/__runa"), SlowThreshold(time.Nanosecond)))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if response := request(app, "/ok"); response.Code != http.StatusOK {
		t.Fatalf("ok = %d %q", response.Code, response.Body.String())
	}
	if response := request(app, "/boom"); response.Code != http.StatusInternalServerError {
		t.Fatalf("boom = %d %q", response.Code, response.Body.String())
	}
	store := MonitorStoreOf(app)
	if access := store.AccessLogs(10); len(access) != 2 || access[0].Path != "/boom" || access[1].Path != "/ok" {
		t.Fatalf("access = %#v", access)
	}
	errors := store.ErrorLogs(10)
	if len(errors) != 1 || errors[0].Path != "/boom" || errors[0].Error == "" {
		t.Fatalf("errors = %#v", errors)
	}
	if slow := store.SlowLogs(10); len(slow) != 2 {
		t.Fatalf("slow = %#v", slow)
	}
}

func TestConsoleMonitorSkipsConsoleRoutes(t *testing.T) {
	app, routes := newConsoleApp()
	routes.Get("/ok", func(ctx *route.Context) error { return ctx.Text("ok") })
	routes.Get("/favicon.ico", func(ctx *route.Context) error { return ctx.SendStatus(http.StatusNoContent) })
	app.Install(Provider(MountAt("/__runa")))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	_ = request(app, "/__runa/overview/api/components")
	_ = request(app, "/favicon.ico")
	if access := MonitorStoreOf(app).AccessLogs(10); len(access) != 0 {
		t.Fatalf("noise access should be skipped: %#v", access)
	}
	_ = request(app, "/ok")
	if access := MonitorStoreOf(app).AccessLogs(10); len(access) != 1 || access[0].Path != "/ok" {
		t.Fatalf("access = %#v", access)
	}
}

func TestComponentTablesExposeFilteringAndPagination(t *testing.T) {
	app, _ := newConsoleApp()
	app.Install(Provider(MountAt("/__runa"), Panels(ComponentPanel{
		Name:  "table",
		Title: "Table",
		Components: []Component{
			{Name: "access", Label: "Access Logs", Type: ComponentTable, Resolve: func(context.Context, AppContext) (any, error) {
				return []core.Map{{"method": "GET", "path": "/ok", "status": 200}, {"method": "POST", "path": "/submit", "status": 500}}, nil
			}},
		},
	})))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	page := request(app, "/__runa/table")
	body := page.Body.String()
	for _, want := range []string{"Filter rows", "All methods", "All status", "25 / page", "Prev", "Next"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}

func TestMemoryMonitorRouteStats(t *testing.T) {
	store := NewMemoryMonitorStore()
	now := time.Now()
	store.RecordAccess(AccessLog{Time: now.Add(-3 * time.Second), Method: http.MethodGet, Path: "/users/1", Route: "users.show", Status: http.StatusOK, Latency: 10 * time.Millisecond})
	store.RecordAccess(AccessLog{Time: now.Add(-2 * time.Second), Method: http.MethodGet, Path: "/users/1", Route: "users.show", Status: http.StatusInternalServerError, Latency: 30 * time.Millisecond, Error: "boom"})
	store.RecordAccess(AccessLog{Time: now.Add(-time.Second), Method: http.MethodPost, Path: "/users", Route: "users.create", Status: http.StatusCreated, Latency: 20 * time.Millisecond})

	stats := store.RouteStats(10)
	if len(stats) != 2 {
		t.Fatalf("stats = %#v", stats)
	}
	first := stats[0]
	if first.Route != "users.show" || first.Method != http.MethodGet || first.Count != 2 || first.Errors != 1 {
		t.Fatalf("first = %#v", first)
	}
	if first.Min != 10*time.Millisecond || first.Max != 30*time.Millisecond || first.Avg != 20*time.Millisecond {
		t.Fatalf("latency = %#v", first)
	}
}

func TestTrafficPanelIncludesRouteStats(t *testing.T) {
	app, routes := newConsoleApp()
	routes.Get("/ok", func(ctx *route.Context) error { return ctx.Text("ok") }).Name("ok")
	app.Install(Provider(MountAt("/__runa")))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	_ = request(app, "/ok")
	response := request(app, "/__runa/traffic/api/components")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{`"routes"`, `"Route Stats"`, `"avg_ms"`, `"max_ms"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}

func TestMemoryMonitorQueueSeries(t *testing.T) {
	store := NewMemoryMonitorStore()
	now := time.Now()
	store.RecordQueue(QueueSample{Time: now.Add(-2 * time.Minute), Pending: 1, Reserved: 1, Processed: 3, Failed: 1})
	store.RecordQueue(QueueSample{Time: now.Add(-time.Minute), Pending: 4, Reserved: 2, Processed: 8, Failed: 3})
	pressure := store.QueuePressureSeries(10*time.Minute, 5)
	if len(pressure) != 5 || sumPoints(pressure) == 0 {
		t.Fatalf("pressure = %#v", pressure)
	}
	throughput := store.WorkerThroughputSeries(10*time.Minute, 5)
	if sumPoints(throughput) != 5 {
		t.Fatalf("throughput = %#v", throughput)
	}
	failures := store.JobFailureSeries(10*time.Minute, 5)
	if sumPoints(failures) != 2 {
		t.Fatalf("failures = %#v", failures)
	}
}

func TestJobsPanelComponents(t *testing.T) {
	app, _ := newConsoleApp()
	app.Install(Provider(MountAt("/__runa")))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := request(app, "/__runa/jobs/api/components")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, name := range []string{`"pressure"`, `"throughput"`, `"failures"`, `"queues"`, `"workers"`} {
		if !strings.Contains(body, name) {
			t.Fatalf("component %s missing in %s", name, body)
		}
	}
}

func TestBuiltinPanelsExposeDashboardSummaries(t *testing.T) {
	app, routes := newConsoleApp()
	routes.Get("/ok", func(ctx *route.Context) error { return ctx.Text("ok") }).Name("ok")
	app.Install(Provider(MountAt("/__runa")))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	_ = request(app, "/ok")
	cases := map[string][]string{
		"/__runa/traffic/api/components":        {`"requests_total"`, `"latency_avg"`, `"Route Stats"`},
		"/__runa/errors/api/components":         {`"total"`, `"rate"`, `"last"`},
		"/__runa/logs/api/components":           {`"access_count"`, `"error_count"`},
		"/__runa/runtime/api/components":        {`"version"`, `"heap"`, `"gc_runs"`},
		"/__runa/routes/api/components":         {`"secured"`, `"middlewares"`},
		"/__runa/schedule/api/components":       {`"enabled"`, `"queued"`, `"handlers"`},
		"/__runa/infrastructure/api/components": {`"modules"`, `"drivers"`, `"defaults"`},
	}
	for path, wants := range cases {
		response := request(app, path)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %q", path, response.Code, response.Body.String())
		}
		body := response.Body.String()
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q in %s", path, want, body)
			}
		}
	}
}

func TestInfrastructureRowsFormatDurations(t *testing.T) {
	app, _ := newConsoleApp()
	app.Install(
		cache.Provider(cache.RegisterPool("slow", cache.TTL(10*time.Minute))),
		session.Provider(session.RegisterSession("long", session.TTL(2*time.Hour))),
		lock.Provider(lock.RegisterLocker("short", lock.TTL(time.Second))),
		rate.Provider(),
		Provider(MountAt("/__runa")),
	)
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := request(app, "/__runa/infrastructure/api/components")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Contains(body, `"ttl":"1"`) || !strings.Contains(body, `"ttl":"10m"`) || !strings.Contains(body, `"ttl":"2h"`) {
		t.Fatalf("bad duration formatting: %s", body)
	}
}

type panelProvider struct{ provider.Base }

func (panelProvider) Name() string { return "panel" }
func (panelProvider) Register(ctx provider.Context) error {
	Register(ctx, PanelFunc{
		Name:  "external",
		Title: "External",
		Icon:  "E",
		Order: 30,
		Mount: func(group *route.Group) {
			group.Get("/ping", func(ctx *route.Context) error { return ctx.Text("pong") })
		},
	})
	return nil
}

type authRegisterProvider struct{ provider.Base }

func (authRegisterProvider) Name() string { return "test.auth.register" }
func (authRegisterProvider) Register(ctx provider.Context) error {
	registry, err := provider.Invoke[*auth.Registry](ctx)
	if err != nil {
		return err
	}
	registry.Auth("console", auth.APIKeyAuth(func(token string) (core.Map, bool, error) {
		return core.Map{"id": "root"}, token == "secret", nil
	}))
	return nil
}

type messageRegisterProvider struct{ provider.Base }

func (messageRegisterProvider) Name() string { return "test.message.register" }
func (messageRegisterProvider) Register(ctx provider.Context) error {
	registry, err := provider.Invoke[*message.Registry](ctx)
	if err != nil {
		return err
	}
	registry.Broker("events")
	registry.Subscribe[map[string]string]("events", "article.created", func(context.Context, *message.MessageOf[map[string]string]) error {
		return nil
	})
	return nil
}

func newConsoleApp(options ...runtime.Option) (*runa.App, *route.Registry) {
	app := runa.New(options...)
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)))
	return app, routes
}

func routesOf(app AppContext) *route.Registry {
	routes, _ := provider.Invoke[*route.Registry](app)
	return routes
}

func request(app AppContext, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	routes := routesOf(app)
	if routes != nil {
		routes.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	}
	return response
}

func sumPoints(items []MetricPoint) float64 {
	var total float64
	for _, item := range items {
		total += item.Value
	}
	return total
}

func TestProtocolPanelsAndExecutionMonitors(t *testing.T) {
	app, _ := newConsoleApp()
	app.Install(message.Provider())
	app.Install(messageRegisterProvider{})
	rpc := jsonrpc.New()
	rpc.Register("ping", func(ctx *jsonrpc.Context) (any, error) { return map[string]string{"ok": "yes"}, nil })
	hub := ws.New("admin", ws.Config{})
	app.Install(jsonrpc.Provider(rpc, jsonrpc.Path("/rpc")))
	app.Install(ws.Provider(hub))
	app.Install(Provider(MountAt("/__runa")))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	messages := provider.MustInvoke[*message.Registry](app)
	if err := messages.Publish(context.Background(), "events", "article.created", map[string]string{"id": "1"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := rpc.Call(context.Background(), "ping", map[string]string{}); err != nil {
		t.Fatalf("rpc: %v", err)
	}
	store := MonitorStoreOf(app)
	if len(store.MessageLogs(10)) == 0 {
		t.Fatal("missing message logs")
	}
	if len(store.RPCLogs(10)) == 0 {
		t.Fatal("missing rpc logs")
	}
	if len(store.WSSamples(10)) == 0 {
		t.Fatal("missing ws samples")
	}
	for _, path := range []string{"/__runa/messages/api/components", "/__runa/websocket/api/components", "/__runa/rpc/api/components"} {
		response := request(app, path)
		if response.Code != http.StatusOK {
			t.Fatalf("%s code = %d body=%s", path, response.Code, response.Body.String())
		}
	}
}
