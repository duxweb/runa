package route

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duxweb/runa/core"
)

type typedInput struct{}
type typedOutput struct {
	Name string `json:"name"`
}

func TestTypedRouteRegisters(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "/api")
	called := false

	Get[typedInput, typedOutput](group, "/typed", func(ctx *Context, input *typedInput) (*typedOutput, error) {
		called = true
		return &typedOutput{Name: "typed"}, nil
	}).Name("typed")

	request := httptest.NewRequest(http.MethodGet, "/api/typed", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "{\"name\":\"typed\"}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
	if !called {
		t.Fatal("typed handler was not called")
	}
	if registry.Routes()[0].RouteName != "typed" {
		t.Fatalf("name = %q", registry.Routes()[0].RouteName)
	}
}

type typedBodyOutput struct {
	Body struct {
		Name string `json:"name"`
	}
}

func TestTypedRouteUsesBodyField(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	Get[typedInput, typedBodyOutput](group, "/body", func(ctx *Context, input *typedInput) (*typedBodyOutput, error) {
		output := &typedBodyOutput{}
		output.Body.Name = "body"
		return output, nil
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/body", nil))
	if response.Body.String() != "{\"name\":\"body\"}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

type typedRawOutput struct {
	Body core.JSONRaw
}

func TestTypedRouteRendersJSONRaw(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	Get[typedInput, typedRawOutput](group, "/raw", func(ctx *Context, input *typedInput) (*typedRawOutput, error) {
		return &typedRawOutput{Body: core.JSONRaw(`{"raw":true}`)}, nil
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/raw", nil))
	if response.Body.String() != `{"raw":true}` {
		t.Fatalf("body = %q", response.Body.String())
	}
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
}

type typedBytesOutput struct {
	Body []byte
}

func TestTypedRouteRendersBytes(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	Get[typedInput, typedBytesOutput](group, "/bytes", func(ctx *Context, input *typedInput) (*typedBytesOutput, error) {
		return &typedBytesOutput{Body: []byte("bytes")}, nil
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/bytes", nil))
	if response.Body.String() != "bytes" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

type typedStreamOutput struct {
	Body core.Stream
}

func TestTypedRouteRendersStream(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	Get[typedInput, typedStreamOutput](group, "/stream", func(ctx *Context, input *typedInput) (*typedStreamOutput, error) {
		return &typedStreamOutput{Body: core.Stream{Reader: strings.NewReader("stream"), ContentType: "text/plain"}}, nil
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/stream", nil))
	if response.Body.String() != "stream" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestTypedRouteEnvelopeAndRaw(t *testing.T) {
	registry := New()
	registry.Envelope(EnvelopeFunc(func(ctx *Context, data any) (any, error) {
		return core.Map{"data": data}, nil
	}))
	group := NewGroup(registry, "")
	Get[typedInput, typedOutput](group, "/wrapped", func(ctx *Context, input *typedInput) (*typedOutput, error) {
		return &typedOutput{Name: "wrapped"}, nil
	})
	Get[typedInput, typedOutput](group, "/raw", func(ctx *Context, input *typedInput) (*typedOutput, error) {
		return &typedOutput{Name: "raw"}, nil
	}).Raw()

	wrapped := httptest.NewRecorder()
	registry.Handler().ServeHTTP(wrapped, httptest.NewRequest(http.MethodGet, "/wrapped", nil))
	if wrapped.Body.String() != "{\"data\":{\"name\":\"wrapped\"}}\n" {
		t.Fatalf("wrapped = %q", wrapped.Body.String())
	}

	raw := httptest.NewRecorder()
	registry.Handler().ServeHTTP(raw, httptest.NewRequest(http.MethodGet, "/raw", nil))
	if raw.Body.String() != "{\"name\":\"raw\"}\n" {
		t.Fatalf("raw = %q", raw.Body.String())
	}
}

func TestTypedRouteStatusAndEmpty(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	Post[typedInput, typedOutput](group, "/created", func(ctx *Context, input *typedInput) (*typedOutput, error) {
		return &typedOutput{Name: "created"}, nil
	}).Status(http.StatusCreated)
	Delete[typedInput, core.Empty](group, "/empty", func(ctx *Context, input *typedInput) (*core.Empty, error) {
		return &core.Empty{}, nil
	})

	created := httptest.NewRecorder()
	registry.Handler().ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/created", nil))
	if created.Code != http.StatusCreated {
		t.Fatalf("created status = %d", created.Code)
	}

	empty := httptest.NewRecorder()
	registry.Handler().ServeHTTP(empty, httptest.NewRequest(http.MethodDelete, "/empty", nil))
	if empty.Code != http.StatusNoContent || empty.Body.String() != "" {
		t.Fatalf("empty = %d %q", empty.Code, empty.Body.String())
	}
}
