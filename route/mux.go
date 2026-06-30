package route

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/scope"
)

func buildRuntime(registry *Registry) (http.Handler, error) {
	mux := http.NewServeMux()
	appPipeline := registry.ErrorPipeline()
	appEnvelope := registry.EnvelopeDef()
	appLang := registry.LangConfig()
	appTranslator := registry.TranslatorDef()
	appConfig := registry.ConfigDef()
	appServices := registry.Services()
	for _, item := range registry.Routes() {
		route := item
		if route.EnvelopeDef == nil && !route.RawResponse {
			route.EnvelopeDef = appEnvelope
		}
		handler := func(writer http.ResponseWriter, request *http.Request) {
			requestScope := scope.New(request.Context(), scope.HTTP)
			defer requestScope.Close()
			recorder := NewStatusRecorder(writer)
			ctx := NewContext(recorder, request, route, routeParams(route, request), requestScope)
			injectContext(ctx, appLang, appTranslator, appConfig, appServices)
			pipeline := route.errorPipeline(appPipeline)
			defer func() {
				if value := recover(); value != nil {
					if recorder.Written() {
						logServerError(ctx, newPanicError(value))
						return
					}
					_ = pipeline.Handle(ctx, newPanicError(value))
				}
			}()
			if err := route.run(ctx); err != nil {
				if recorder.Written() {
					logServerError(ctx, err)
					return
				}
				_ = pipeline.Handle(ctx, err)
			}
		}
		if route.Method == "ANY" {
			if err := handle(mux, route.Path, handler); err != nil {
				return nil, err
			}
			continue
		}
		if err := handle(mux, route.Method+" "+route.Path, handler); err != nil {
			return nil, err
		}
	}
	return muxHandler{
		mux:        mux,
		pipeline:   appPipeline,
		lang:       appLang,
		translator: appTranslator,
		config:     appConfig,
		services:   appServices,
	}, nil
}

func handle(mux *http.ServeMux, pattern string, handler http.HandlerFunc) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("route pattern %q: %v", pattern, value)
		}
	}()
	mux.HandleFunc(pattern, handler)
	return nil
}

func routeParams(route *Route, request *http.Request) map[string]string {
	params := make(map[string]string)
	for _, name := range paramNames(route.Path) {
		params[name] = request.PathValue(name)
	}
	return params
}

func paramNames(path string) []string {
	items := make([]string, 0)
	for _, part := range strings.Split(path, "/") {
		if !strings.HasPrefix(part, "{") || !strings.HasSuffix(part, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
		name = strings.TrimSuffix(name, "...")
		if name != "" && name != "$" {
			items = append(items, name)
		}
	}
	return items
}

type muxHandler struct {
	mux        *http.ServeMux
	pipeline   ErrorPipeline
	lang       Lang
	translator Translator
	config     *config.Store
	services   map[string]any
}

func (handler muxHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	_, pattern := handler.mux.Handler(request)
	if pattern != "" {
		handler.mux.ServeHTTP(writer, request)
		return
	}
	requestScope := scope.New(request.Context(), scope.HTTP)
	defer requestScope.Close()
	ctx := NewContext(writer, request, nil, nil, requestScope)
	injectContext(ctx, handler.lang, handler.translator, handler.config, handler.services)
	if allow := allowedMethods(handler.mux, request); allow != "" {
		writer.Header().Set("Allow", allow)
		_ = handler.pipeline.Handle(ctx, methodNotAllowedError{})
		return
	}
	_ = handler.pipeline.Handle(ctx, notFoundError{})
}

func allowedMethods(mux *http.ServeMux, request *http.Request) string {
	probe := request.Clone(request.Context())
	methods := make([]string, 0, 8)
	seen := make(map[string]bool)
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		probe.Method = method
		_, pattern := mux.Handler(probe)
		if pattern == "" {
			continue
		}
		if method == "HEAD" && seen["GET"] {
			continue
		}
		if !seen[method] {
			seen[method] = true
			methods = append(methods, method)
		}
	}
	return strings.Join(methods, ", ")
}

func injectContext(
	ctx *Context,
	lang Lang,
	translator Translator,
	config *config.Store,
	services map[string]any,
) {
	ctx.setLangRuntime(lang, translator)
	ctx.config = config
	ctx.services = services
}
