package route

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runlog "github.com/duxweb/runa/log"
	"github.com/duxweb/runa/runtime"
)

func TestDefaultErrorResponseHidesInternalError(t *testing.T) {
	var logs bytes.Buffer
	app, routes := newLoggedRouteApp(&logs)
	routes.Get("/boom", func(ctx *Context) error {
		return errors.New("database password leaked")
	})
	if err := app.Freeze(app.Context()); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/boom", nil)
	response := httptest.NewRecorder()
	routes.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "Internal Server Error" {
		t.Fatalf("body = %q", response.Body.String())
	}
	logBody := logs.String()
	if !strings.Contains(logBody, `"level":"ERROR"`) || !strings.Contains(logBody, "database password leaked") {
		t.Fatalf("log = %q", logBody)
	}
}

func TestPanicUsesErrorPipeline(t *testing.T) {
	var logs bytes.Buffer
	app, routes := newLoggedRouteApp(&logs)
	routes.Get("/", func(ctx *Context) error { panic("boom") })
	if err := app.Freeze(app.Context()); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	routes.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "Internal Server Error" {
		t.Fatalf("body = %q", response.Body.String())
	}
	logBody := logs.String()
	if !strings.Contains(logBody, "panic: boom") || !strings.Contains(logBody, `"stack":`) || !strings.Contains(logBody, `"source":`) {
		t.Fatalf("panic log = %q", logBody)
	}
}

func TestContextErrorLogsSourceAndStack(t *testing.T) {
	var logs bytes.Buffer
	app, routes := newLoggedRouteApp(&logs)
	routes.Get("/", func(ctx *Context) error {
		return ctx.Error(http.StatusInternalServerError, "boom")
	})
	if err := app.Freeze(app.Context()); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	response := httptest.NewRecorder()
	routes.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	logBody := logs.String()
	if !strings.Contains(logBody, `"source":`) || !strings.Contains(logBody, "error_integration_test.go") || !strings.Contains(logBody, `"stack":`) {
		t.Fatalf("log = %q", logBody)
	}
}

func newLoggedRouteApp(logs *bytes.Buffer) (*runtime.App, *Registry) {
	app := runtime.New()
	routes := New()
	app.Install(Provider(UseRegistry(routes)), runlog.Provider(runlog.Register(runlog.Error, runlog.Writer(logs, runlog.JSON()))))
	return app, routes
}
