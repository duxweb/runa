package route

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/errs"
)

func TestContextLangAndTranslate(t *testing.T) {
	registry := New()
	registry.Lang(Lang{
		Default: "zh-CN",
		Sources: []LangSource{
			LangSourceFunc(func(request *http.Request) string {
				return request.URL.Query().Get("lang")
			}),
		},
	})
	registry.Translator(TranslatorFunc(func(ctx *Context, lang string, message string, params core.Map) string {
		if lang == "en-US" {
			return "en:" + message + ":" + core.Cast[string](params["name"])
		}
		return lang + ":" + message
	}))
	group := NewGroup(registry, "")
	group.Get("/lang", func(ctx *Context) error {
		return ctx.Text(ctx.T("hello", core.Map{"name": "runa"}))
	})

	request := httptest.NewRequest(http.MethodGet, "/lang?lang=en-US", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Body.String() != "en:hello:runa" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestContextSetLangOverridesSources(t *testing.T) {
	registry := New()
	registry.Lang(Lang{
		Default: "zh-CN",
		Sources: []LangSource{LangSourceFunc(func(request *http.Request) string {
			return request.Header.Get("Accept-Language")
		})},
	})
	group := NewGroup(registry, "")
	group.Get("/lang", func(ctx *Context) error {
		ctx.SetLang("ja-JP")
		return ctx.Text(ctx.Lang())
	})

	request := httptest.NewRequest(http.MethodGet, "/lang", nil)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)
	if response.Body.String() != "ja-JP" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestTranslatorUsesDefaultLangAndParamsInErrors(t *testing.T) {
	registry := New()
	registry.Lang(Lang{Default: "zh-CN"})
	registry.OnError(func(ctx *Context, err error) error {
		if base := errs.As(err); base != nil {
			return ctx.Error(http.StatusBadRequest, err)
		}
		return err
	})
	registry.Translator(TranslatorFunc(func(ctx *Context, lang string, message string, params core.Map) string {
		return lang + ":" + strings.ReplaceAll(message, "{name}", core.Cast[string](params["name"]))
	}))
	group := NewGroup(registry, "")
	group.Get("/error", func(ctx *Context) error {
		return errs.New("你好 {name}", errs.Attr("name", "Runa"))
	})

	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/error", nil))
	if response.Code != http.StatusBadRequest || response.Body.String() != "zh-CN:你好 Runa" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}
