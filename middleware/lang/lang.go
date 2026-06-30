package lang

import (
	"net/http"
	"strings"

	runalang "github.com/duxweb/runa/lang"
	"github.com/duxweb/runa/route"
)

// Source resolves a locale from an HTTP request.
type Source interface {
	Resolve(*http.Request) string
}

// SourceFunc adapts a function to Source.
type SourceFunc func(*http.Request) string

// Resolve resolves a locale.
func (fn SourceFunc) Resolve(request *http.Request) string {
	if fn == nil {
		return ""
	}
	return fn(request)
}

// Query reads locale from query string.
func Query(name string) Source {
	return SourceFunc(func(request *http.Request) string {
		if request == nil || request.URL == nil {
			return ""
		}
		return clean(request.URL.Query().Get(name))
	})
}

// Cookie reads locale from a cookie.
func Cookie(name string) Source {
	return SourceFunc(func(request *http.Request) string {
		if request == nil {
			return ""
		}
		cookie, err := request.Cookie(name)
		if err != nil {
			return ""
		}
		return clean(cookie.Value)
	})
}

// Header reads locale from a header. Accept-Language values are passed through for i18n matching.
func Header(name string) Source {
	return SourceFunc(func(request *http.Request) string {
		if request == nil {
			return ""
		}
		return clean(request.Header.Get(name))
	})
}

// New creates a route middleware that negotiates the request locale and stores a translator in context.
func New(sources ...Source) route.Middleware {
	if len(sources) == 0 {
		sources = []Source{Query("lang"), Cookie("lang"), Header("Accept-Language")}
	}
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			preferences := resolve(ctx.Request(), sources)
			translator := runalang.Default().Translator(preferences...)
			if locale := translator.Locale(); locale != "" {
				ctx.SetLang(locale)
			}
			ctx.SetContext(runalang.WithTranslator(ctx.Context(), translator))
			return next(ctx)
		}
	}
}

func resolve(request *http.Request, sources []Source) []string {
	items := make([]string, 0, len(sources))
	for _, source := range sources {
		if source == nil {
			continue
		}
		if value := clean(source.Resolve(request)); value != "" {
			items = append(items, value)
		}
	}
	return items
}

func clean(value string) string { return strings.TrimSpace(value) }
