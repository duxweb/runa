package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/duxweb/runa/route"
)

func TestCORSAllowsOrigin(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{AllowOrigins: []string{"https://admin.example.com"}, Credentials: true, ExposeHeaders: []string{"X-Total"}}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://admin.example.com")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Allow-Origin") != "https://admin.example.com" {
		t.Fatalf("allow origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
	if response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("credentials = %q", response.Header().Get("Access-Control-Allow-Credentials"))
	}
	if response.Header().Get("Access-Control-Expose-Headers") != "X-Total" {
		t.Fatalf("expose = %q", response.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestCORSPreflight(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{AllowOrigins: []string{"https://admin.example.com"}, MaxAge: 600}))
	group.Options("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text("should not run")
	})

	request := httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "https://admin.example.com")
	request.Header.Set("Access-Control-Request-Method", "POST")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "" {
		t.Fatalf("body = %q", response.Body.String())
	}
	if response.Header().Get("Access-Control-Max-Age") != "600" {
		t.Fatalf("max age = %q", response.Header().Get("Access-Control-Max-Age"))
	}
}

func TestCORSRejectsOrigin(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{AllowOrigins: []string{"https://admin.example.com"}}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://evil.example.com")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("allow origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSWildcardCredentialsDoesNotReflectOrigin(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{AllowOrigins: []string{"*"}, Credentials: true}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://evil.example.com")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("allow origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
	if response.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("credentials = %q", response.Header().Get("Access-Control-Allow-Credentials"))
	}
}

func TestCORSNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Next: func(*route.Context) bool { return true }}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://admin.example.com")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("allow origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
}
