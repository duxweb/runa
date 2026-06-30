package route

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppRouteHandler(t *testing.T) {
	app := New()
	app.Get("/ping", func(ctx *Context) error {
		return ctx.Status(http.StatusOK).Text("pong")
	}).Name("ping")

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "pong" {
		t.Fatalf("body = %q", response.Body.String())
	}
	if len(app.Routes()) != 1 || app.Routes()[0].RouteName != "ping" {
		t.Fatalf("routes = %#v", app.Routes())
	}
}

func TestAppGroupRoute(t *testing.T) {
	app := New()
	app.Group("/api", func(group *Group) {
		group.Get("/ping", func(ctx *Context) error {
			return ctx.Status(http.StatusOK).Text("api")
		})
	})

	request := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "api" {
		t.Fatalf("body = %q", response.Body.String())
	}
}
