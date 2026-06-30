package route

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/duxweb/runa/core"
)

// Lang configures request language resolution.
type Lang struct {
	Default string
	Sources []LangSource
}

// LangSource resolves a language from an HTTP request.
type LangSource interface {
	Resolve(*http.Request) string
}

// LangSourceFunc adapts a function to LangSource.
type LangSourceFunc func(*http.Request) string

// Resolve resolves the language.
func (fn LangSourceFunc) Resolve(request *http.Request) string {
	if fn == nil {
		return ""
	}
	return fn(request)
}

// Translator translates messages for the current request.
type Translator interface {
	Translate(*Context, string, string, core.Map) string
}

// TranslatorFunc adapts a function to Translator.
type TranslatorFunc func(*Context, string, string, core.Map) string

// Translate translates a message.
func (fn TranslatorFunc) Translate(ctx *Context, currentLang string, message string, params core.Map) string {
	if fn == nil {
		return replaceMessage(message, params)
	}
	return fn(ctx, currentLang, message, params)
}

// SetLang sets the current request language.
func (ctx *Context) SetLang(value string) *Context {
	ctx.lang = strings.TrimSpace(value)
	return ctx
}

// Lang returns the current request language.
func (ctx *Context) Lang() string {
	if ctx.lang != "" {
		return ctx.lang
	}
	if value := resolveLang(ctx.request, ctx.langSources); value != "" {
		ctx.lang = value
		return ctx.lang
	}
	return ctx.defaultLang
}

// T translates a message using current request language.
func (ctx *Context) T(message string, params ...core.Map) string {
	translator := ctx.translator
	if translator == nil {
		return replaceMessage(message, firstMap(params...))
	}
	return translator.Translate(ctx, ctx.Lang(), message, firstMap(params...))
}

func (ctx *Context) setLangRuntime(config Lang, translator Translator) {
	ctx.defaultLang = strings.TrimSpace(config.Default)
	ctx.langSources = append([]LangSource(nil), config.Sources...)
	ctx.translator = translator
}

func resolveLang(request *http.Request, sources []LangSource) string {
	for _, source := range sources {
		if source == nil {
			continue
		}
		if value := strings.TrimSpace(source.Resolve(request)); value != "" {
			return value
		}
	}
	return ""
}

func replaceMessage(message string, params core.Map) string {
	if message == "" || len(params) == 0 {
		return message
	}
	pairs := make([]string, 0, len(params)*2)
	for key, value := range params {
		pairs = append(pairs, "{"+key+"}", fmt.Sprint(value))
	}
	return strings.NewReplacer(pairs...).Replace(message)
}
