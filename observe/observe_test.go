package observe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/database"
	"github.com/duxweb/runa/host"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/runtime"
	"github.com/duxweb/runa/storage"
)

type fakeDriver struct{ db *fakeDB }

func (driver fakeDriver) Open(context.Context, database.Config) (database.Database, error) {
	return driver.db, nil
}

type fakeDB struct{}

func (db *fakeDB) Name() string                { return "default" }
func (db *fakeDB) Kind() string                { return "fake" }
func (db *fakeDB) Raw() any                    { return db }
func (db *fakeDB) Ping(context.Context) error  { return nil }
func (db *fakeDB) Close(context.Context) error { return nil }
func (db *fakeDB) Info() database.Info {
	return database.Info{Name: "default", Kind: "fake", Dialect: "memory"}
}

type fakeHost struct{ status host.Status }

func (unit *fakeHost) Name() string                { return "worker" }
func (unit *fakeHost) Start(context.Context) error { unit.status = host.Running; return nil }
func (unit *fakeHost) Stop(context.Context) error  { unit.status = host.Stopped; return nil }
func (unit *fakeHost) Status() host.Status         { return unit.status }

func TestProviderMountsHealthAndReady(t *testing.T) {
	app := runa.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)), database.Provider(database.RegisterDriver("default", fakeDriver{db: &fakeDB{}})), cache.Provider(), queue.Provider(), storage.Provider())
	app.Host(&fakeHost{status: host.Running})
	app.Install(Provider(Config{Service: "admin", Mount: "/debug"},
		Ready("database", Database(app, "default")),
		Ready("cache", Cache(app, "default")),
		Ready("queue", Queue(app, "default")),
		Ready("storage", Storage(app, "")),
	))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	health := request(routes, "/debug/health")
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"status":"pass"`) {
		t.Fatalf("health code=%d body=%s", health.Code, health.Body.String())
	}
	ready := request(routes, "/debug/ready")
	if ready.Code != http.StatusOK || !strings.Contains(ready.Body.String(), `"database"`) {
		t.Fatalf("ready code=%d body=%s", ready.Code, ready.Body.String())
	}
	monitor := request(routes, "/debug/debug/monitor")
	if monitor.Code != http.StatusNotFound {
		t.Fatalf("monitor code=%d body=%s", monitor.Code, monitor.Body.String())
	}
}

func TestProviderReadsConfig(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "observe.toml", `mount = "/ops"
debug = true
`)
	app := runtime.New(runtime.BasePath(root))
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)), Provider(Config{}))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	health := request(routes, "/ops/health")
	if health.Code != http.StatusOK {
		t.Fatalf("health code=%d body=%s", health.Code, health.Body.String())
	}
	monitor := request(routes, "/ops/debug/monitor")
	if monitor.Code != http.StatusOK || !strings.Contains(monitor.Body.String(), `"runtime"`) {
		t.Fatalf("monitor code=%d body=%s", monitor.Code, monitor.Body.String())
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

func TestReadyFailureReturnsUnavailable(t *testing.T) {
	app := runtime.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)), Provider(Config{}, Ready("broken", CheckerFunc(func(context.Context) Result {
		return failed("broken", errors.New("down"), nil)
	}))))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := request(routes, "/ready")
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"status":"fail"`) {
		t.Fatalf("ready code=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCheckersReadAppRuntime(t *testing.T) {
	app := runa.New()
	app.Install(database.Provider(database.RegisterDriver("default", fakeDriver{db: &fakeDB{}})), cache.Provider(), queue.Provider(), storage.Provider())
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	ctx := context.Background()
	for _, checker := range []Checker{Self(), Database(app, "default"), Cache(app, "default"), Queue(app, "default"), Storage(app, "")} {
		if result := checker.Check(ctx); result.Status != Pass {
			t.Fatalf("%s = %#v", checker.Name(), result)
		}
	}
}

func TestMetricsAndDebugEndpoints(t *testing.T) {
	app := runtime.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)), Provider(Config{Debug: true}, Metrics(TextMetrics("runa_test 1"))))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	metrics := request(routes, "/metrics")
	if metrics.Code != http.StatusOK || !strings.Contains(metrics.Body.String(), "runa_test 1") {
		t.Fatalf("metrics code=%d body=%s", metrics.Code, metrics.Body.String())
	}
	vars := request(routes, "/debug/vars")
	if vars.Code != http.StatusOK || !strings.Contains(vars.Body.String(), "cmdline") {
		t.Fatalf("vars code=%d body=%s", vars.Code, vars.Body.String())
	}
	monitor := request(routes, "/debug/monitor")
	if monitor.Code != http.StatusOK || !strings.Contains(monitor.Body.String(), `"runtime"`) {
		t.Fatalf("monitor code=%d body=%s", monitor.Code, monitor.Body.String())
	}
}

func TestRegistryTimeoutAndPanic(t *testing.T) {
	registry := New()
	registry.Add("slow", CheckerFunc(func(ctx context.Context) Result {
		<-ctx.Done()
		return ok("slow", "late", nil)
	}))
	registry.Add("panic", CheckerFunc(func(context.Context) Result { panic("boom") }))
	report := registry.Run(context.Background(), time.Millisecond)
	if report.Status != Fail || len(report.Results) != 2 {
		t.Fatalf("report = %#v", report)
	}
}

func request(routes *route.Registry, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	routes.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response
}
