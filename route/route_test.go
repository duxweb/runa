package route

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestGroupRegisterAndMetadata(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "/api")

	route := group.Get("/users/{id}", func(ctx *Context) error {
		return ctx.Status(http.StatusOK).Text(ctx.Param[string]("id"))
	}).Name("user.show").Meta("permission", "system.user").Summary("用户详情").Tags("User")

	if route.Path != "/api/users/{id}" {
		t.Fatalf("path = %q", route.Path)
	}
	if route.RouteName != "user.show" {
		t.Fatalf("name = %q", route.RouteName)
	}
	if got := route.MetaAs[string]("permission"); got != "system.user" {
		t.Fatalf("permission = %q", got)
	}
	if route.SummaryText != "用户详情" {
		t.Fatalf("summary = %q", route.SummaryText)
	}
	if !reflect.DeepEqual(route.TagList, []string{"User"}) {
		t.Fatalf("tags = %#v", route.TagList)
	}
}

func TestChiHandlerParamAndQuery(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/users/{id}", func(ctx *Context) error {
		return ctx.Status(http.StatusOK).JSON(map[string]any{
			"id":   ctx.Param[int]("id"),
			"name": ctx.Query[string]("name"),
		})
	})

	request := httptest.NewRequest(http.MethodGet, "/users/12?name=runa", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if body := response.Body.String(); body != "{\"id\":12,\"name\":\"runa\"}\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestMiddlewareOrder(t *testing.T) {
	calls := []string{}
	registry := New()
	group := NewGroup(registry, "")
	group.Use(func(next Handler) Handler {
		return func(ctx *Context) error {
			calls = append(calls, "group:before")
			err := next(ctx)
			calls = append(calls, "group:after")
			return err
		}
	})
	group.Get("/ping", func(ctx *Context) error {
		calls = append(calls, "handler")
		return ctx.Status(http.StatusOK).Text("ok")
	}).Use(func(next Handler) Handler {
		return func(ctx *Context) error {
			calls = append(calls, "route:before")
			err := next(ctx)
			calls = append(calls, "route:after")
			return err
		}
	})

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	expected := []string{"group:before", "route:before", "handler", "route:after", "group:after"}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("calls = %#v, want %#v", calls, expected)
	}
}

func TestRouteOverride(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/ping", func(ctx *Context) error {
		return ctx.Status(http.StatusOK).Text("old")
	})
	group.Get("/ping", func(ctx *Context) error {
		return ctx.Status(http.StatusOK).Text("new")
	})

	if len(registry.Routes()) != 1 {
		t.Fatalf("routes len = %d", len(registry.Routes()))
	}

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if body := response.Body.String(); body != "new" {
		t.Fatalf("body = %q", body)
	}
}

func TestHTTPScopeClose(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	cleaned := false
	group.Get("/scope", func(ctx *Context) error {
		ctx.Locals("name", "runa")
		ctx.Scope().OnClose(func(ctx context.Context) error {
			cleaned = true
			return nil
		})
		return ctx.Status(http.StatusOK).Text(ctx.Locals("name").(string))
	})

	request := httptest.NewRequest(http.MethodGet, "/scope", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "runa" {
		t.Fatalf("body = %q", response.Body.String())
	}
	if !cleaned {
		t.Fatal("scope cleanup was not called")
	}
}

func TestContextMetaUsesCast(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/meta", func(ctx *Context) error {
		return ctx.JSON(map[string]any{
			"enabled": ctx.Meta[bool]("enabled"),
			"items":   ctx.Meta[[]int]("items"),
		})
	}).Meta("enabled", "on").Meta("items", "1,2,3")

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/meta", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Body.String() != "{\"enabled\":true,\"items\":[1,2,3]}\n" {
		t.Fatalf("body=%q", response.Body.String())
	}
}

func TestServeMuxCatchAllParam(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/files/{path...}", func(ctx *Context) error {
		return ctx.Text(ctx.Param[string]("path"))
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/files/a/b/c.txt", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Body.String() != "a/b/c.txt" {
		t.Fatalf("body=%q", response.Body.String())
	}
}

func TestServeMuxMethodNotAllowedUsesErrorPipeline(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/only-get", func(ctx *Context) error {
		return ctx.Text("ok")
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/only-get", nil))

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Body.String() != "Method Not Allowed" {
		t.Fatalf("body=%q", response.Body.String())
	}
	if response.Header().Get("Allow") == "" {
		t.Fatalf("allow header missing: %v", response.Header())
	}
}
