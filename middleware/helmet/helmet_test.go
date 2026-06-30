package helmet

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/duxweb/runa/route"
)

func TestHelmetSetsDefaultHeaders(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New())
	group.Get("/", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("ok") })

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("nosniff = %q", response.Header().Get("X-Content-Type-Options"))
	}
	if response.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatalf("frame = %q", response.Header().Get("X-Frame-Options"))
	}
}

func TestHelmetCustomHeaders(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Custom: map[string]string{"Permissions-Policy": "geolocation=()"}}))
	group.Get("/", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("ok") })

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Header().Get("Permissions-Policy") != "geolocation=()" {
		t.Fatalf("permissions = %q", response.Header().Get("Permissions-Policy"))
	}
}

func TestHelmetNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Next: func(*route.Context) bool { return true }}))
	group.Get("/", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("ok") })

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Header().Get("X-Content-Type-Options") != "" {
		t.Fatalf("nosniff = %q", response.Header().Get("X-Content-Type-Options"))
	}
}
