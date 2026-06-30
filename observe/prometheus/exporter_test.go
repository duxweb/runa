package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/observe"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/route"
)

func TestExporter(t *testing.T) {
	exporter := Exporter("runa_custom 1")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	ctx := route.NewContext(response, request, nil, nil)
	if err := exporter.Serve(ctx); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(response.Body.String(), "runa_custom 1") {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestAppExporter(t *testing.T) {
	app := runa.New()
	routes := route.New()
	app.Install(queue.Provider(
		queue.RegisterQueue("mail", queue.Workers("mail")),
		queue.RegisterWorker("mail"),
	))
	app.Install(route.Provider(route.UseRegistry(routes)))
	routes.Get("/ping", func(ctx *route.Context) error { return ctx.Text("pong") })
	app.Install(observe.Provider(observe.Config{}, observe.Metrics(Exporter(app))))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	response := httptest.NewRecorder()
	routes.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, want := range []string{
		"# HELP runa_info",
		"runa_routes_total",
		"runa_runtime_goroutines",
		"runa_queue_jobs",
		"runa_worker_processed_total",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}

func TestHTTPCollector(t *testing.T) {
	app := runa.New()
	routes := route.New()
	app.Install(route.Provider(route.UseRegistry(routes)))
	routes.Get("/ping", func(ctx *route.Context) error { return ctx.Text("pong") })
	routes.Get("/boom", func(ctx *route.Context) error { return ctx.Status(http.StatusInternalServerError).Text("boom") })
	app.Install(observe.Provider(observe.Config{},
		observe.Metrics(Exporter(app)),
		observe.Trace(HTTPCollector()),
	))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	handler := routes.Handler()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, want := range []string{
		`runa_http_requests_total{method="GET",path="/ping",status="200"} 1`,
		`runa_http_errors_total{method="GET",path="/boom",status="500"} 1`,
		`runa_http_request_duration_seconds_bucket{le="+Inf",method="GET",path="/ping",status="200"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
	if strings.Contains(body, `path="/metrics"`) {
		t.Fatalf("observe endpoint was collected: %s", body)
	}
}
