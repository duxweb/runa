package route

import (
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/validate"
)

type bindInput struct {
	ID     int    `path:"id" label:"ID"`
	Page   int    `query:"page" default:"1"`
	Token  string `bind:"header:Authorization,cookie:token"`
	Name   string `form:"name"`
	Detail bool   `query:"detail"`
}

func (input *bindInput) Validate(v *validate.Validator) {
	v.Field("ID").Required("ID 必填")
	v.Field("Token").Required("Token 必填")
}

func TestContextInputBindsAndValidates(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Post("/users/{id}", func(ctx *Context) error {
		input, err := Input[bindInput](ctx)
		if err != nil {
			return err
		}
		if input.ID != 12 || input.Page != 2 || input.Token != "Bearer token" || input.Name != "Runa" || !input.Detail {
			t.Fatalf("input = %#v", input)
		}
		return ctx.Status(http.StatusOK).Text("ok")
	})

	request := httptest.NewRequest(http.MethodPost, "/users/12?page=2&detail=true", strings.NewReader("name=Runa"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestContextInputValidationError(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/users/{id}", func(ctx *Context) error {
		_, err := Input[bindInput](ctx)
		return err
	})

	request := httptest.NewRequest(http.MethodGet, "/users/12", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "Token 必填" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

type typedBindInput struct {
	ID int `path:"id"`
}

type typedBindOutput struct{}

func TestTypedRouteBindsInput(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	Get[typedBindInput, typedBindOutput](group, "/users/{id}", func(ctx *Context, input *typedBindInput) (*typedBindOutput, error) {
		if input.ID != 44 {
			t.Fatalf("id = %d", input.ID)
		}
		return &typedBindOutput{}, nil
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/44", nil))
	if response.Code != http.StatusOK || response.Body.String() != "{}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

type scalarBindInput struct {
	Age      int8          `query:"age"`
	Timeout  time.Duration `query:"timeout"`
	Datetime time.Time     `query:"datetime"`
}

func TestContextInputRejectsOverflowAndBindsTimeScalars(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/scalar", func(ctx *Context) error {
		input, err := Input[scalarBindInput](ctx)
		if err != nil {
			return err
		}
		if input.Age != 12 || input.Timeout != 30*time.Second || input.Datetime.IsZero() {
			t.Fatalf("input = %#v", input)
		}
		return ctx.Text("ok")
	})

	ok := httptest.NewRecorder()
	registry.Handler().ServeHTTP(ok, httptest.NewRequest(http.MethodGet, "/scalar?age=12&timeout=30s&datetime=2026-01-02T03:04:05Z", nil))
	if ok.Code != http.StatusOK {
		t.Fatalf("ok status=%d body=%q", ok.Code, ok.Body.String())
	}

	overflow := httptest.NewRecorder()
	registry.Handler().ServeHTTP(overflow, httptest.NewRequest(http.MethodGet, "/scalar?age=300", nil))
	if overflow.Code != http.StatusBadRequest {
		t.Fatalf("overflow status=%d body=%q", overflow.Code, overflow.Body.String())
	}
}

type jsonInput struct {
	Title string       `json:"title"`
	Raw   core.JSONRaw `json:"raw"`
}

func TestContextInputBindsJSON(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Post("/json", func(ctx *Context) error {
		input, err := Input[jsonInput](ctx)
		if err != nil {
			return err
		}
		if input.Title != "Runa" || input.Raw.String() != `{"level":1}` {
			t.Fatalf("input = %#v raw=%s", input, input.Raw.String())
		}
		return ctx.Status(http.StatusOK).Text("json")
	})

	request := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader(`{"title":"Runa","raw":{"level":1}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "json" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

type bodyInput struct {
	Body struct {
		Title string `json:"title"`
	} `body:"json"`
}

func TestContextInputBindsBodyField(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Post("/body", func(ctx *Context) error {
		input, err := Input[bodyInput](ctx)
		if err != nil {
			return err
		}
		if input.Body.Title != "Body" {
			t.Fatalf("body = %#v", input.Body)
		}
		return ctx.Status(http.StatusOK).Text("body")
	})

	request := httptest.NewRequest(http.MethodPost, "/body", strings.NewReader(`{"title":"Body"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "body" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

type rawBodyInput struct {
	Body []byte `body:"bytes"`
}

func TestContextInputBindsRawBody(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Post("/raw", func(ctx *Context) error {
		input, err := Input[rawBodyInput](ctx)
		if err != nil {
			return err
		}
		if string(input.Body) != "raw-body" {
			t.Fatalf("body = %q", string(input.Body))
		}
		return ctx.Status(http.StatusOK).Text("raw")
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/raw", strings.NewReader("raw-body")))
	if response.Code != http.StatusOK || response.Body.String() != "raw" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

type streamInput struct {
	Body core.Stream `body:"stream"`
}

func TestContextInputBindsStreamBody(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Post("/stream", func(ctx *Context) error {
		input, err := Input[streamInput](ctx)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(input.Body.Reader)
		if err != nil {
			return err
		}
		if string(body) != "stream-body" {
			t.Fatalf("stream = %q", string(body))
		}
		return ctx.Status(http.StatusOK).Text("stream")
	})

	request := httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader("stream-body"))
	request.Header.Set("Content-Type", "application/octet-stream")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "stream" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

type fileInput struct {
	File core.UploadFile `file:"file"`
}

type fileSliceInput struct {
	Files []*core.UploadFile `file:"file"`
}

func TestContextInputBindsFile(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Post("/upload", func(ctx *Context) error {
		input, err := Input[fileInput](ctx)
		if err != nil {
			return err
		}
		if input.File.Filename != "runa.txt" || input.File.Size == 0 {
			t.Fatalf("file = %#v", input.File)
		}
		return ctx.Status(http.StatusOK).Text("file")
	})

	var body strings.Builder
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "runa.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("runa"))
	_ = writer.Close()

	request := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body.String()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "file" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestContextInputBindsFileSlice(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Post("/uploads", func(ctx *Context) error {
		input, err := Input[fileSliceInput](ctx)
		if err != nil {
			return err
		}
		if len(input.Files) != 2 || input.Files[0].Filename != "one.txt" || input.Files[1].Filename != "two.txt" {
			t.Fatalf("files = %#v", input.Files)
		}
		return ctx.Status(http.StatusOK).Text("files")
	})

	var body strings.Builder
	writer := multipart.NewWriter(&body)
	first, err := writer.CreateFormFile("file", "one.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = first.Write([]byte("one"))
	second, err := writer.CreateFormFile("file", "two.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = second.Write([]byte("two"))
	_ = writer.Close()

	request := httptest.NewRequest(http.MethodPost, "/uploads", strings.NewReader(body.String()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "files" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}
