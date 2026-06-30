package route

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/errs"
	"github.com/duxweb/runa/validate"
)

func TestBaseErrorHelpers(t *testing.T) {
	cause := errors.New("cause")
	err := errs.New("missing", errs.Code("user.missing"), errs.Attr("id", 12), errs.Cause(cause))
	baseErr := errs.As(err)
	if baseErr == nil {
		t.Fatal("base error is nil")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("cause not wrapped: %v", err)
	}
	if baseErr.Code != "user.missing" {
		t.Fatalf("code = %q", baseErr.Code)
	}
	if baseErr.Params["id"] != 12 {
		t.Fatalf("params = %#v", baseErr.Params)
	}
	if ErrorStatus(err) != http.StatusInternalServerError {
		t.Fatalf("status = %d", ErrorStatus(err))
	}
	if ErrorMessage(err) != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("message = %q", ErrorMessage(err))
	}
	var traced interface {
		Source() string
		StackTrace() string
	}
	if !errors.As(err, &traced) {
		t.Fatal("base error does not expose trace")
	}
	if !strings.Contains(traced.Source(), "errors_pipeline_test.go") || !strings.Contains(traced.StackTrace(), "errors_pipeline_test.go") {
		t.Fatalf("trace source=%q stack=%q", traced.Source(), traced.StackTrace())
	}
}

func TestValidationErrorHelpers(t *testing.T) {
	err := validate.Invalid(validate.FieldError{Field: "name", Code: "required", Message: "请输入名称"})
	if ErrorStatus(err) != http.StatusBadRequest {
		t.Fatalf("status = %d", ErrorStatus(err))
	}
	if ErrorCode(err) != "validation_error" {
		t.Fatalf("code = %q", ErrorCode(err))
	}
	if ErrorMessage(err) != "请输入名称" {
		t.Fatalf("message = %q", ErrorMessage(err))
	}
	if validate.AsError(err) == nil {
		t.Fatal("validation error is nil")
	}
}

func TestDefaultTextErrorResponse(t *testing.T) {
	app := New()
	app.Get("/missing", func(ctx *Context) error {
		return ctx.Error(http.StatusNotFound, "没有找到")
	})

	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "没有找到" {
		t.Fatalf("body = %q", response.Body.String())
	}
	if !strings.HasPrefix(response.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
}

func TestBaseErrorResponseDefaultsToInternal(t *testing.T) {
	app := New()
	app.Get("/boom", func(ctx *Context) error {
		return errs.New("业务错误", errs.Code("service.failed"))
	})

	request := httptest.NewRequest(http.MethodGet, "/boom", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "Internal Server Error" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestContextErrorMapsBaseErrorToHTTP(t *testing.T) {
	app := New()
	baseErr := errs.New("用户不存在", errs.Code("user.not_found"))
	app.Get("/missing", func(ctx *Context) error {
		return ctx.Error(http.StatusNotFound, baseErr)
	})

	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "用户不存在" {
		t.Fatalf("body = %q", response.Body.String())
	}

	app.Get("/inspect", func(ctx *Context) error {
		routeErr := ctx.Error(http.StatusNotFound, baseErr)
		if !errors.Is(routeErr, baseErr) {
			t.Fatalf("route error does not wrap base error: %v", routeErr)
		}
		return nil
	})
	app.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/inspect", nil))
}

func TestContextErrorMessageVariants(t *testing.T) {
	app := New()
	app.Get("/text", func(ctx *Context) error {
		return ctx.Error(http.StatusForbidden, "没有权限")
	})
	app.Get("/default", func(ctx *Context) error {
		return ctx.Error(http.StatusNotFound, nil)
	})

	text := httptest.NewRecorder()
	app.Handler().ServeHTTP(text, httptest.NewRequest(http.MethodGet, "/text", nil))
	if text.Code != http.StatusForbidden || text.Body.String() != "没有权限" {
		t.Fatalf("text = %d %q", text.Code, text.Body.String())
	}

	def := httptest.NewRecorder()
	app.Handler().ServeHTTP(def, httptest.NewRequest(http.MethodGet, "/default", nil))
	if def.Code != http.StatusNotFound || def.Body.String() != http.StatusText(http.StatusNotFound) {
		t.Fatalf("default = %d %q", def.Code, def.Body.String())
	}
}

func TestAppOnErrorNormalizesError(t *testing.T) {
	app := New()
	app.OnError(func(ctx *Context, err error) error {
		return ctx.Error(http.StatusBadRequest, "normalized")
	})
	app.Get("/", func(ctx *Context) error {
		return errors.New("bad")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "normalized" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestRouteOnErrorOverridesApp(t *testing.T) {
	app := New()
	app.OnError(func(ctx *Context, err error) error {
		return ctx.Error(http.StatusBadRequest, "app")
	})
	app.Get("/", func(ctx *Context) error {
		return errors.New("bad")
	}).OnError(func(ctx *Context, err error) error {
		return ctx.Error(http.StatusForbidden, "route")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "route" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestJSONErrorRenderer(t *testing.T) {
	app := New()
	app.Error(JSONErrorRenderer{})
	app.Get("/", func(ctx *Context) error {
		return ctx.Error(http.StatusUnauthorized, "请登录")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "{\"code\":\"unauthorized\",\"message\":\"请登录\"}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestNegotiatedErrorRenderer(t *testing.T) {
	app := New()
	app.Error(NegotiatedErrorRenderer{})
	app.Get("/", func(ctx *Context) error {
		return ctx.Error(http.StatusBadRequest, "bad")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
	if response.Body.String() != "{\"code\":\"bad_request\",\"message\":\"bad\"}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestNotFoundAndMethodNotAllowedUseErrorPipeline(t *testing.T) {
	app := New()
	app.Get("/exists", func(ctx *Context) error { return ctx.Status(http.StatusOK).Text("ok") })

	missing := httptest.NewRecorder()
	app.Handler().ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if missing.Code != http.StatusNotFound || missing.Body.String() != "Not Found" {
		t.Fatalf("missing = %d %q", missing.Code, missing.Body.String())
	}

	method := httptest.NewRecorder()
	app.Handler().ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/exists", nil))
	if method.Code != http.StatusMethodNotAllowed || method.Body.String() != "Method Not Allowed" {
		t.Fatalf("method = %d %q", method.Code, method.Body.String())
	}
}

func TestEnvelopeWrapsJSONResponse(t *testing.T) {
	app := New()
	app.Envelope(EnvelopeFunc(func(ctx *Context, data any) (any, error) {
		return core.Map{"code": 0, "data": data}, nil
	}))
	app.Get("/", func(ctx *Context) error {
		return ctx.Status(http.StatusOK).JSON(core.Map{"name": "runa"})
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Body.String() != "{\"code\":0,\"data\":{\"name\":\"runa\"}}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestRouteRawSkipsEnvelope(t *testing.T) {
	app := New()
	app.Envelope(EnvelopeFunc(func(ctx *Context, data any) (any, error) {
		return core.Map{"code": 0, "data": data}, nil
	}))
	app.Get("/", func(ctx *Context) error {
		return ctx.Status(http.StatusOK).JSON(core.Map{"name": "runa"})
	}).Raw()

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Body.String() != "{\"name\":\"runa\"}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

type thirdPartyNotFound struct{}

func (thirdPartyNotFound) Error() string { return "record not found" }

func TestThirdPartyErrorDefaultsToInternal(t *testing.T) {
	app := New()
	app.Get("/", func(ctx *Context) error {
		return thirdPartyNotFound{}
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "Internal Server Error" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestOnErrorMapsThirdPartyError(t *testing.T) {
	app := New()
	app.OnError(func(ctx *Context, err error) error {
		var notFound thirdPartyNotFound
		if errors.As(err, &notFound) {
			return ctx.Error(http.StatusNotFound, "record missing")
		}
		return err
	})
	app.Get("/", func(ctx *Context) error {
		return thirdPartyNotFound{}
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "record missing" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestOnErrorMapsBaseErrorCode(t *testing.T) {
	app := New()
	app.OnError(func(ctx *Context, err error) error {
		baseErr := errs.As(err)
		if baseErr != nil && baseErr.Code == "user.not_found" {
			return ctx.Error(http.StatusNotFound, err)
		}
		return err
	})
	app.Get("/", func(ctx *Context) error {
		return errs.New("用户不存在", errs.Code("user.not_found"))
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "用户不存在" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestXMLAndHTMLErrorRenderer(t *testing.T) {
	xmlApp := New()
	xmlApp.Error(XMLErrorRenderer{})
	xmlApp.Get("/", func(ctx *Context) error { return ctx.Error(http.StatusBadRequest, "bad") })
	xmlResponse := httptest.NewRecorder()
	xmlApp.Handler().ServeHTTP(xmlResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(xmlResponse.Body.String(), "<message>bad</message>") {
		t.Fatalf("xml body = %q", xmlResponse.Body.String())
	}

	htmlApp := New()
	htmlApp.Error(HTMLErrorRenderer{})
	htmlApp.Get("/", func(ctx *Context) error { return ctx.Error(http.StatusBadRequest, "<bad>") })
	htmlResponse := httptest.NewRecorder()
	htmlApp.Handler().ServeHTTP(htmlResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if htmlResponse.Body.String() != "<h1>&lt;bad&gt;</h1>" {
		t.Fatalf("html body = %q", htmlResponse.Body.String())
	}
}

func TestGroupEnvelope(t *testing.T) {
	app := New()
	app.Group("/admin", func(group *Group) {
		group.Envelope(EnvelopeFunc(func(ctx *Context, data any) (any, error) {
			return core.Map{"admin": true, "data": data}, nil
		}))
		group.Get("/user", func(ctx *Context) error {
			return ctx.Status(http.StatusOK).JSON(core.Map{"name": "runa"})
		})
	})

	request := httptest.NewRequest(http.MethodGet, "/admin/user", nil)
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)

	if response.Body.String() != "{\"admin\":true,\"data\":{\"name\":\"runa\"}}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestWrapErrorUsesOops(t *testing.T) {
	base := errors.New("base")
	wrapped := errs.Wrap(base)
	if !errors.Is(wrapped, base) {
		t.Fatalf("wrapped error does not contain base: %v", wrapped)
	}
}
