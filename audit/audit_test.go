package audit_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/audit"
	auditmiddleware "github.com/duxweb/runa/audit/middleware"
	"github.com/duxweb/runa/auth"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/runtime"
)

func TestAuditMiddlewareWritesEntry(t *testing.T) {
	var entries []audit.Entry
	app, routes := newRouteApp()
	routes.Group("/admin", func(group *route.Group) {
		group.Name("admin")
		group.Use(auditmiddleware.New(audit.Config{Mode: audit.Sync, CaptureInput: true, Writer: audit.FuncWriter(func(ctx context.Context, entry audit.Entry) error {
			entries = append(entries, entry)
			return nil
		})}))
		group.Post("/users/{id}", func(ctx *route.Context) error {
			ctx.Locals("runa.auth", &auth.Info{Name: "admin", Data: core.Map{"id": 12, "name": "root"}})
			return ctx.Status(http.StatusCreated).Text("ok")
		}).Name("users.edit").Meta("action", "edit").Meta("resource", "user")
	})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/admin/users/1?keyword=runa", strings.NewReader(`{"name":"Runa","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	routes.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated || len(entries) != 1 {
		t.Fatalf("code=%d entries=%d", response.Code, len(entries))
	}
	entry := entries[0]
	if entry.Route != "admin.users.edit" || entry.Action != "edit" || entry.Status != http.StatusCreated || !entry.Success {
		t.Fatalf("entry = %#v", entry)
	}
	if entry.ActorID != "12" || entry.ActorName != "root" || entry.Guard != "admin" {
		t.Fatalf("actor = %#v", entry)
	}
	if entry.Input["password"] != "***" || entry.Input["keyword"] != "runa" {
		t.Fatalf("input = %#v", entry.Input)
	}
	if entry.Meta["resource"] != "user" {
		t.Fatalf("meta = %#v", entry.Meta)
	}
}

func TestAuditSkipsMethodsAndNext(t *testing.T) {
	count := 0
	app, routes := newRouteApp()
	routes.Group("/api", func(group *route.Group) {
		group.Use(auditmiddleware.New(
			audit.Config{Mode: audit.Sync, Writer: audit.FuncWriter(func(context.Context, audit.Entry) error {
				count++
				return nil
			})},
			auditmiddleware.Next(func(ctx *route.Context) bool {
				return route.Query[string](ctx, "skip") == "1"
			}),
		))
		group.Get("/users", func(ctx *route.Context) error { return ctx.Text("get") })
		group.Post("/users", func(ctx *route.Context) error { return ctx.Text("post") })
	})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	serve(routes, http.MethodGet, "/api/users", "")
	serve(routes, http.MethodPost, "/api/users?skip=1", "")
	serve(routes, http.MethodPost, "/api/users", "")
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
}

func TestAuditStrictSyncReturnsWriteError(t *testing.T) {
	app, routes := newRouteApp()
	routes.RouteGroup().Use(auditmiddleware.New(audit.Config{Mode: audit.Sync, Strict: true, Writer: audit.FuncWriter(func(context.Context, audit.Entry) error {
		return errors.New("audit down")
	})}))
	routes.Post("/save", func(ctx *route.Context) error { return ctx.Text("ok") })
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := serve(routes, http.MethodPost, "/save", "")
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "Internal Server Error") {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestAuditProviderRegistersRegistry(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "audit.toml", `mode = "sync"
capture_input = true
methods = ["POST"]
`)
	app, routes := newRunaRouteApp(runa.BasePath(root))
	var entries []audit.Entry
	app.Install(audit.Provider(audit.Config{Writer: audit.FuncWriter(func(ctx context.Context, entry audit.Entry) error {
		entries = append(entries, entry)
		return nil
	})}))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	registry := audit.Default()
	if registry == nil || registry.Config().Mode != audit.Sync || !registry.Config().CaptureInput {
		t.Fatalf("registry = %#v", registry)
	}
	routes.RouteGroup().Use(auditmiddleware.Use(registry))
	routes.Post("/save", func(ctx *route.Context) error { return ctx.Text("ok") })
	response := serve(routes, http.MethodPost, "/save?password=query", "name=runa")
	if response.Code != http.StatusOK || len(entries) != 1 {
		t.Fatalf("response=%d entries=%d", response.Code, len(entries))
	}
	if entries[0].Input["password"] != "***" || entries[0].Input["name"] != "runa" {
		t.Fatalf("input = %#v", entries[0].Input)
	}
}

func TestAuditQueueWriterAndHandleQueue(t *testing.T) {
	app := runa.New()
	app.Install(queue.Provider(
		queue.RegisterQueue("audit", queue.Workers("audit")),
		queue.RegisterWorker("audit"),
	))
	var received []audit.Entry
	var mu sync.Mutex
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	queues := queue.Default()
	audit.HandleQueue(queues, func(ctx context.Context, entry audit.Entry) error {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, entry)
		return nil
	})
	writer := audit.QueueWriter(queues, "audit")
	if err := writer.Write(context.Background(), audit.Entry{Action: "test"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- queues.Work(ctx, "audit") }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("work: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0].Action != "test" {
		t.Fatalf("received = %#v", received)
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

func TestAuditMaskMap(t *testing.T) {
	masked := audit.Mask(core.Map{
		"password": "secret",
		"profile":  core.Map{"token": "abc", "name": "runa"},
	}, audit.DefaultMaskFields(), "***")
	if masked["password"] != "***" || masked["profile"].(core.Map)["token"] != "***" {
		t.Fatalf("masked = %#v", masked)
	}
}

func TestAuditCaptureInputFalseAndFormMasking(t *testing.T) {
	var entries []audit.Entry
	app, routes := newRouteApp()
	routes.RouteGroup().Use(auditmiddleware.New(audit.Config{Mode: audit.Sync, CaptureInput: false, Writer: audit.FuncWriter(func(ctx context.Context, entry audit.Entry) error {
		entries = append(entries, entry)
		return nil
	})}))
	routes.Post("/off", func(ctx *route.Context) error { return ctx.Text("ok") })
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := serve(routes, http.MethodPost, "/off?password=query", "password=body")
	if response.Code != http.StatusOK || len(entries) != 1 {
		t.Fatalf("code=%d entries=%d", response.Code, len(entries))
	}
	if len(entries[0].Input) != 0 {
		t.Fatalf("input = %#v", entries[0].Input)
	}

	entries = nil
	app, routes = newRouteApp()
	routes.RouteGroup().Use(auditmiddleware.New(audit.Config{Mode: audit.Sync, CaptureInput: true, Writer: audit.FuncWriter(func(ctx context.Context, entry audit.Entry) error {
		entries = append(entries, entry)
		return nil
	})}))
	routes.Post("/form", func(ctx *route.Context) error { return ctx.Text("ok") })
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("password=secret&name=runa"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	routes.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || len(entries) != 1 {
		t.Fatalf("code=%d entries=%d", recorder.Code, len(entries))
	}
	if entries[0].Input["password"] != "***" || entries[0].Input["name"] != "runa" {
		t.Fatalf("input = %#v", entries[0].Input)
	}
}

func newRouteApp() (*runtime.App, *route.Registry) {
	app := runtime.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)))
	return app, routes
}

func newRunaRouteApp(options ...runtime.Option) (*runtime.App, *route.Registry) {
	app := runa.New(options...)
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)))
	return app, routes
}

func serve(routes *route.Registry, method string, path string, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" && method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	routes.Handler().ServeHTTP(response, request)
	return response
}
