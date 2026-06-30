package lang

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	runalang "github.com/duxweb/runa/lang"
	"github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/route"
	"github.com/samber/do/v2"
)

func TestMiddlewareInjectsTranslator(t *testing.T) {
	registry := runalang.New(runalang.DefaultLocale("en"))
	provider.SetDefaultInjector(do.New())
	provider.ProvideValueOnceTo(provider.DefaultInjector(), registry)

	mw := New(Header("Accept-Language"))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	ctx := route.NewContext(httptest.NewRecorder(), request, nil, nil)
	called := false
	err := mw(func(ctx *route.Context) error {
		called = true
		if runalang.From(ctx.Context()) == nil {
			t.Fatalf("translator missing")
		}
		if ctx.Lang() == "" {
			t.Fatalf("lang missing")
		}
		return nil
	})(ctx)
	if err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if !called {
		t.Fatalf("handler not called")
	}
}

func TestResolveSources(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/?lang=zh", nil)
	request.AddCookie(&http.Cookie{Name: "lang", Value: "en"})
	items := resolve(request, []Source{Query("lang"), Cookie("lang")})
	if len(items) != 2 || items[0] != "zh" || items[1] != "en" {
		t.Fatalf("items = %#v", items)
	}
	_ = context.Background()
}
