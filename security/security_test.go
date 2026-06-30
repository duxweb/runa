package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/duxweb/runa/route"
)

func TestSecurityDefaultWritesHeadersAndRequestID(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Disable("logger")))
	group.Get("/ping", func(ctx *route.Context) error {
		if ctx.RequestID() == "" {
			t.Fatal("expected request id")
		}
		return ctx.Text(ctx.IP())
	})

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.10")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "203.0.113.10" {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing helmet headers: %v", response.Header())
	}
}

func TestSecurityDisableAndNext(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Disable("helmet", "logger"), Next(func(ctx *route.Context) bool {
		return ctx.Request().URL.Path == "/skip"
	})))
	group.Get("/skip", func(ctx *route.Context) error { return ctx.Text("skip") })
	group.Get("/normal", func(ctx *route.Context) error { return ctx.Text("normal") })

	skip := httptest.NewRecorder()
	registry.Handler().ServeHTTP(skip, httptest.NewRequest(http.MethodGet, "/skip", nil))
	if skip.Header().Get("X-Request-ID") != "" || skip.Header().Get("X-Content-Type-Options") != "" {
		t.Fatalf("next should skip headers: %v", skip.Header())
	}
	normal := httptest.NewRecorder()
	registry.Handler().ServeHTTP(normal, httptest.NewRequest(http.MethodGet, "/normal", nil))
	if normal.Header().Get("X-Request-ID") == "" || normal.Header().Get("X-Content-Type-Options") != "" {
		t.Fatalf("disable helmet but keep request id: %v", normal.Header())
	}
}

func TestSecurityOptions(t *testing.T) {
	config := defaultConfig()
	Development()(&config)
	if !config.RecoverStack || config.TimeoutValue != 120*time.Second {
		t.Fatalf("development config = %+v", config)
	}
	Production()(&config)
	if config.RecoverStack || len(config.TrustedProxies) != 0 {
		t.Fatalf("production config = %+v", config)
	}
	if parseSize("2MB") != 2<<20 || parseSize("1KB") != 1<<10 || parseSize("bad") != 32<<20 {
		t.Fatal("parseSize failed")
	}
}
