package realip

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/duxweb/runa/route"
)

func TestRealIPUsesForwardedHeaderFromTrustedProxy(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New())
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.IP())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.10, 127.0.0.1")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "203.0.113.10" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestRealIPSkipsTrustedProxyChainFromRight(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{TrustedProxies: []string{"127.0.0.1", "10.0.0.0/8"}}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.IP())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.10, 10.0.0.2")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "203.0.113.10" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestRealIPIgnoresForwardedHeaderFromUntrustedProxy(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New())
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.IP())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "198.51.100.2:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.10")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "198.51.100.2" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestRealIPSetsSchemeAndHost(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New())
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.Scheme() + "://" + ctx.Hostname())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "admin.example.com:8443")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "https://admin.example.com" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestRealIPNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Next: func(*route.Context) bool { return true }}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.IP())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.10")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "127.0.0.1" {
		t.Fatalf("body = %q", response.Body.String())
	}
}
