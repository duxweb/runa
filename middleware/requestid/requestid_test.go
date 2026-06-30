package requestid

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/duxweb/runa/route"
)

func TestRequestIDGeneratesAndStores(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Generator: func() string { return "generated" }}))
	group.Get("/", func(ctx *route.Context) error {
		if ctx.RequestID() != "generated" {
			t.Fatalf("request id = %q", ctx.RequestID())
		}
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Header().Get("X-Request-ID") != "generated" {
		t.Fatalf("header = %q", response.Header().Get("X-Request-ID"))
	}
}

func TestRequestIDReusesHeader(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Generator: func() string { return "generated" }}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.RequestID())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "incoming")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "incoming" {
		t.Fatalf("body = %q", response.Body.String())
	}
	if response.Header().Get("X-Request-ID") != "incoming" {
		t.Fatalf("header = %q", response.Header().Get("X-Request-ID"))
	}
}

func TestRequestIDRejectsInvalidHeader(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Generator: func() string { return "generated" }}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.RequestID())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", string(make([]byte, 129)))
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "generated" {
		t.Fatalf("body = %q", response.Body.String())
	}
	if response.Header().Get("X-Request-ID") != "generated" {
		t.Fatalf("header = %q", response.Header().Get("X-Request-ID"))
	}
}

func TestRequestIDFallsBackWhenGeneratorReturnsEmpty(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Generator: func() string { return "" }}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.RequestID())
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Body.String() == "" {
		t.Fatal("request id should not be empty")
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("response request id should not be empty")
	}
}

func TestRequestIDNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Next: func(*route.Context) bool { return true }, Generator: func() string { return "generated" }}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.RequestID())
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "" {
		t.Fatalf("body = %q", response.Body.String())
	}
	if response.Header().Get("X-Request-ID") != "" {
		t.Fatalf("header = %q", response.Header().Get("X-Request-ID"))
	}
}
