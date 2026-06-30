package healthcheck

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/duxweb/runa/route"
)

func TestHealthcheckResponds(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New())
	group.Get("/health", func(ctx *route.Context) error { return ctx.Status(http.StatusInternalServerError).Text("bad") })

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "ok" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestHealthcheckPassesThrough(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New())
	group.Get("/ping", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("pong") })

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "pong" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestHealthcheckNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Next: func(*route.Context) bool { return true }}))
	group.Get("/health", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("custom") })

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "custom" {
		t.Fatalf("body = %q", response.Body.String())
	}
}
