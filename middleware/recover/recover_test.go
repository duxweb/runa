package recover

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/duxweb/runa/route"
)

func TestRecoverConvertsPanicToError(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New())
	group.Get("/", func(ctx *route.Context) error {
		panic("boom")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "Internal Server Error" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestRecoverOnRecover(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	expected := errors.New("custom")
	group.Use(New(Config{Stack: true, OnRecover: func(ctx *route.Context, value any, stack []byte) error {
		if value != "boom" {
			t.Fatalf("value = %v", value)
		}
		if len(stack) == 0 {
			t.Fatal("stack is empty")
		}
		return expected
	}}))
	group.Get("/", func(ctx *route.Context) error {
		panic("boom")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "Internal Server Error" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestRecoverDefaultErrorCarriesStack(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	var recovered error
	group.OnError(func(ctx *route.Context, err error) error {
		recovered = err
		return err
	})
	group.Use(New())
	group.Get("/", func(ctx *route.Context) error {
		panic("boom")
	})

	registry.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var traced interface {
		Source() string
		Stack() []byte
	}
	if !errors.As(recovered, &traced) {
		t.Fatal("panic error does not expose stack")
	}
	if len(traced.Stack()) == 0 || traced.Source() == "" {
		t.Fatalf("source=%q stack=%d", traced.Source(), len(traced.Stack()))
	}
}

func TestRecoverStackDisabled(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	var recovered error
	group.OnError(func(ctx *route.Context, err error) error {
		recovered = err
		return err
	})
	group.Use(New(Config{Stack: false}))
	group.Get("/", func(ctx *route.Context) error {
		panic("boom")
	})

	registry.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var traced interface {
		Source() string
		Stack() []byte
	}
	if !errors.As(recovered, &traced) {
		t.Fatal("panic error does not expose trace methods")
	}
	if len(traced.Stack()) != 0 || traced.Source() != "" {
		t.Fatalf("source=%q stack=%d", traced.Source(), len(traced.Stack()))
	}
}

func TestRecoverNextSkips(t *testing.T) {
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Next: func(*route.Context) bool { return true }}))
	group.Get("/", func(ctx *route.Context) error {
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "ok" {
		t.Fatalf("body = %q", response.Body.String())
	}
}
