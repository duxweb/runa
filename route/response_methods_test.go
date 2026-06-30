package route

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContextJsonUsesPendingStatus(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/json", func(ctx *Context) error {
		return ctx.Status(http.StatusCreated).JSON(map[string]string{"name": "runa"})
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/json", nil))
	if response.Code != http.StatusCreated || response.Body.String() != "{\"name\":\"runa\"}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestContextSendAndText(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/send", func(ctx *Context) error {
		return ctx.Send([]byte("bytes"))
	})
	group.Get("/string", func(ctx *Context) error {
		return ctx.Status(http.StatusAccepted).Text("text")
	})

	bytesResponse := httptest.NewRecorder()
	registry.Handler().ServeHTTP(bytesResponse, httptest.NewRequest(http.MethodGet, "/send", nil))
	if bytesResponse.Body.String() != "bytes" || bytesResponse.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("send = %d %q %q", bytesResponse.Code, bytesResponse.Body.String(), bytesResponse.Header().Get("Content-Type"))
	}

	stringResponse := httptest.NewRecorder()
	registry.Handler().ServeHTTP(stringResponse, httptest.NewRequest(http.MethodGet, "/string", nil))
	if stringResponse.Code != http.StatusAccepted || stringResponse.Body.String() != "text" {
		t.Fatalf("string = %d %q", stringResponse.Code, stringResponse.Body.String())
	}
}

func TestContextBlobHTMLStreamAndRedirect(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/blob", func(ctx *Context) error {
		return ctx.Blob("image/png", []byte("png"))
	})
	group.Get("/html", func(ctx *Context) error {
		return ctx.Status(http.StatusCreated).HTML("<b>ok</b>")
	})
	group.Get("/stream", func(ctx *Context) error {
		return ctx.SendStream(strings.NewReader("stream"), "text/plain")
	})
	group.Get("/redirect", func(ctx *Context) error {
		return ctx.Redirect("/target", http.StatusMovedPermanently)
	})

	blob := httptest.NewRecorder()
	registry.Handler().ServeHTTP(blob, httptest.NewRequest(http.MethodGet, "/blob", nil))
	if blob.Header().Get("Content-Type") != "image/png" || blob.Body.String() != "png" {
		t.Fatalf("blob = %q %q", blob.Header().Get("Content-Type"), blob.Body.String())
	}

	html := httptest.NewRecorder()
	registry.Handler().ServeHTTP(html, httptest.NewRequest(http.MethodGet, "/html", nil))
	if html.Code != http.StatusCreated || html.Header().Get("Content-Type") != "text/html; charset=utf-8" || html.Body.String() != "<b>ok</b>" {
		t.Fatalf("html = %d %q %q", html.Code, html.Header().Get("Content-Type"), html.Body.String())
	}

	stream := httptest.NewRecorder()
	registry.Handler().ServeHTTP(stream, httptest.NewRequest(http.MethodGet, "/stream", nil))
	if stream.Header().Get("Content-Type") != "text/plain" || stream.Body.String() != "stream" {
		t.Fatalf("stream = %q %q", stream.Header().Get("Content-Type"), stream.Body.String())
	}

	redirect := httptest.NewRecorder()
	registry.Handler().ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/redirect", nil))
	if redirect.Code != http.StatusMovedPermanently || redirect.Header().Get("Location") != "/target" {
		t.Fatalf("redirect = %d %q", redirect.Code, redirect.Header().Get("Location"))
	}
}

func TestContextTypeUsesAliasesAndKeepsSingleResponseStyle(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/json", func(ctx *Context) error {
		return ctx.Type("json").Send([]byte(`{"ok":true}`))
	})
	group.Get("/text", func(ctx *Context) error {
		return ctx.Status(http.StatusAccepted).Type("txt").Text("ok")
	})

	jsonResponse := httptest.NewRecorder()
	registry.Handler().ServeHTTP(jsonResponse, httptest.NewRequest(http.MethodGet, "/json", nil))
	if jsonResponse.Header().Get("Content-Type") != "application/json; charset=utf-8" || jsonResponse.Body.String() != `{"ok":true}` {
		t.Fatalf("json = %q %q", jsonResponse.Header().Get("Content-Type"), jsonResponse.Body.String())
	}

	textResponse := httptest.NewRecorder()
	registry.Handler().ServeHTTP(textResponse, httptest.NewRequest(http.MethodGet, "/text", nil))
	if textResponse.Code != http.StatusAccepted || textResponse.Header().Get("Content-Type") != "text/plain; charset=utf-8" || textResponse.Body.String() != "ok" {
		t.Fatalf("text = %d %q %q", textResponse.Code, textResponse.Header().Get("Content-Type"), textResponse.Body.String())
	}
}

func TestHandlerErrorAfterResponseWrittenDoesNotRenderAgain(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/written", func(ctx *Context) error {
		if err := ctx.Text("ok"); err != nil {
			return err
		}
		return errors.New("late error")
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/written", nil))
	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestSecureCookiePrefixForcesSecure(t *testing.T) {
	registry := New()
	group := NewGroup(registry, "")
	group.Get("/cookie", func(ctx *Context) error {
		ctx.SetCookie("__Secure-token", "value")
		return ctx.Text("ok")
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/cookie", nil))
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("cookies = %#v", cookies)
	}
}
