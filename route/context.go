package route

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"runtime"
	"strings"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/scope"
)

// Context is Runa's HTTP request context.
type Context struct {
	writer  http.ResponseWriter
	request *http.Request
	route   *Route
	params  map[string]string
	scope   *scope.Scope
	status  int

	lang        string
	defaultLang string
	langSources []LangSource
	translator  Translator
	config      *config.Store
	services    map[string]any
	savers      []func(context.Context) error
	viewDomain  string
}

// NewContext creates a route context.
func NewContext(writer http.ResponseWriter, request *http.Request, route *Route, params map[string]string, requestScope ...*scope.Scope) *Context {
	if params == nil {
		params = make(map[string]string)
	}
	currentScope := firstScope(requestScope...)
	if currentScope == nil {
		currentScope = scope.New(request.Context(), scope.HTTP)
	}
	return &Context{
		writer:  writer,
		request: request,
		route:   route,
		params:  params,
		scope:   currentScope,
	}
}

// Request returns the underlying request.
func (ctx *Context) Request() *http.Request { return ctx.request }

// Body reads the request body and restores it for later reads.
func (ctx *Context) Body() ([]byte, error) {
	if ctx.request == nil || ctx.request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(ctx.request.Body)
	if err != nil {
		return nil, err
	}
	ctx.request.Body = io.NopCloser(strings.NewReader(string(body)))
	return body, nil
}

// Context returns the request context.
func (ctx *Context) Context() context.Context {
	if ctx.request == nil {
		return context.Background()
	}
	return ctx.request.Context()
}

// SetContext replaces the underlying request context.
func (ctx *Context) SetContext(value context.Context) *Context {
	if value == nil || ctx.request == nil {
		return ctx
	}
	ctx.request = ctx.request.WithContext(value)
	return ctx
}

// Response returns the underlying response writer.
func (ctx *Context) Response() http.ResponseWriter { return ctx.writer }

// Route returns the matched route metadata.
func (ctx *Context) Route() *Route { return ctx.route }

// Meta reads current route metadata as any.
func (ctx *Context) Meta(key string) any {
	if ctx.route == nil || ctx.route.MetaData == nil {
		return nil
	}
	return ctx.route.MetaData[key]
}

// Meta reads current route metadata cast to T.
func Meta[T any](ctx *Context, key string, fallback ...T) T {
	if ctx == nil || ctx.route == nil {
		return core.Cast[T](nil, fallback...)
	}
	return MetaAs[T](ctx.route, key, fallback...)
}

// Scope returns the request scope.
func (ctx *Context) Scope() *scope.Scope { return ctx.scope }

// ViewDomain sets the request-local default view domain.
func (ctx *Context) ViewDomain(name string) *Context {
	ctx.viewDomain = name
	return ctx
}

// Config returns the app config store or a named config view.
func (ctx *Context) Config(name ...string) *config.Store {
	if len(name) > 0 && name[0] != "" {
		return ctx.config.Scope(name[0])
	}
	return ctx.config
}

// AddSaver registers a response-time saver.
func (ctx *Context) AddSaver(save func(context.Context) error) {
	ctx.savers = append(ctx.savers, save)
}

// ParamString returns a path parameter as string.
func (ctx *Context) ParamString(name string) string { return Param[string](ctx, name) }

// Param returns a path parameter cast to T.
func Param[T any](ctx *Context, name string, fallback ...T) T {
	if ctx == nil {
		return core.Cast[T](nil, fallback...)
	}
	return core.Cast[T](ctx.params[name], fallback...)
}

// QueryString returns a query value as string.
func (ctx *Context) QueryString(name string) string { return Query[string](ctx, name) }

// Query returns a query value cast to T.
func Query[T any](ctx *Context, name string, fallback ...T) T {
	if ctx == nil || ctx.request == nil {
		return core.Cast[T](nil, fallback...)
	}
	return core.Cast[T](ctx.request.URL.Query().Get(name), fallback...)
}

// HeaderString returns a request header as string.
func (ctx *Context) HeaderString(name string) string { return Header[string](ctx, name) }

// Header returns a request header cast to T.
func Header[T any](ctx *Context, name string, fallback ...T) T {
	if ctx == nil || ctx.request == nil {
		return core.Cast[T](nil, fallback...)
	}
	return core.Cast[T](ctx.request.Header.Get(name), fallback...)
}

// Cookie returns a cookie value cast to T.
func Cookie[T any](ctx *Context, name string, fallback ...T) T {
	if ctx == nil || ctx.request == nil {
		return core.Cast[T](nil, fallback...)
	}
	cookie, err := ctx.request.Cookie(name)
	if err != nil {
		return core.Cast[T](nil, fallback...)
	}
	return core.Cast[T](cookie.Value, fallback...)
}

// CookieValue returns a raw cookie value.
func (ctx *Context) CookieValue(name string) (string, bool) {
	if ctx.request == nil {
		return "", false
	}
	cookie, err := ctx.request.Cookie(name)
	if err != nil {
		return "", false
	}
	return cookie.Value, true
}

// Form returns a form value cast to T.
func Form[T any](ctx *Context, name string, fallback ...T) T {
	if ctx == nil || ctx.request == nil {
		return core.Cast[T](nil, fallback...)
	}
	if err := ctx.request.ParseMultipartForm(32 << 20); err != nil && err != http.ErrNotMultipart {
		return core.Cast[T](nil, fallback...)
	}
	return core.Cast[T](ctx.request.FormValue(name), fallback...)
}

// File returns the first uploaded file for a form field.
func (ctx *Context) File(name string) (*multipart.FileHeader, bool) {
	if ctx.request == nil {
		return nil, false
	}
	if err := ctx.request.ParseMultipartForm(32 << 20); err != nil && err != http.ErrNotMultipart {
		return nil, false
	}
	if ctx.request.MultipartForm == nil || ctx.request.MultipartForm.File == nil {
		return nil, false
	}
	files := ctx.request.MultipartForm.File[name]
	if len(files) == 0 {
		return nil, false
	}
	return files[0], true
}

// Error creates a route-layer HTTP error.
func (ctx *Context) Error(status int, value any, params ...core.Map) error {
	err := &HTTPError{
		Status: status,
		Params: firstMap(params...),
		frames: captureFrames(1),
	}
	switch typed := value.(type) {
	case nil:
		err.Message = http.StatusText(status)
	case string:
		err.Message = typed
	case error:
		err.Cause = typed
		err.Message = typed.Error()
		if err.Params == nil {
			var params interface{ ErrorParams() core.Map }
			if errors.As(typed, &params) {
				err.Params = params.ErrorParams()
			}
		}
		var code interface{ ErrorCode() string }
		if errors.As(typed, &code) {
			err.Code = code.ErrorCode()
		}
	default:
		err.Message = fmt.Sprint(typed)
	}
	return err
}

func captureFrames(skip int) []runtime.Frame {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return nil
	}
	iterator := runtime.CallersFrames(pcs[:n])
	frames := make([]runtime.Frame, 0, n)
	for {
		frame, more := iterator.Next()
		if !strings.Contains(frame.Function, "github.com/duxweb/runa/route.captureFrames") {
			frames = append(frames, frame)
		}
		if !more {
			break
		}
	}
	return frames
}

// Locals gets or sets request-local data.
func (ctx *Context) Locals(key string, value ...any) any {
	if len(value) > 0 {
		ctx.scope.Set(key, value[0])
	}
	return ctx.scope.Get(key)
}

// Status sets the response status for the next response write.
func (ctx *Context) Status(code int) *Context {
	ctx.status = code
	return ctx
}

// StatusCode returns the pending or route default success status.
func (ctx *Context) StatusCode(fallback ...int) int {
	if ctx.status > 0 {
		return ctx.status
	}
	if ctx.route != nil && ctx.route.SuccessStatus > 0 {
		return ctx.route.SuccessStatus
	}
	if len(fallback) > 0 && fallback[0] > 0 {
		return fallback[0]
	}
	return http.StatusOK
}

// Set sets a response header.
func (ctx *Context) Set(name string, value string) *Context {
	ctx.writer.Header().Set(name, value)
	return ctx
}

// Type sets the response content type.
func (ctx *Context) Type(contentType string) *Context {
	if resolved := resolveContentType(contentType); resolved != "" {
		ctx.Set("Content-Type", resolved)
	}
	return ctx
}

// SendStatus writes an empty response with status.
func (ctx *Context) SendStatus(code int) error {
	if err := ctx.saveSessions(); err != nil {
		return err
	}
	ctx.writer.WriteHeader(code)
	return nil
}

// Send writes a bytes response using the pending status.
func (ctx *Context) Send(body []byte) error {
	return ctx.write(body, core.MIMEOctetStream)
}

// Text writes a plain text response using the pending status.
func (ctx *Context) Text(body string) error {
	return ctx.write([]byte(body), core.MIMETextPlain)
}

// HTML writes an HTML response using the pending status.
func (ctx *Context) HTML(body string) error {
	return ctx.write([]byte(body), core.MIMETextHTML)
}

// Render renders a template response.
func (ctx *Context) Render(name string, data any, views ...string) error {
	renderer := Service[Renderer](ctx)
	if renderer == nil {
		return fmt.Errorf("route renderer is not configured")
	}
	domain := ""
	if len(views) > 0 && views[0] != "" {
		domain = views[0]
	} else if ctx.viewDomain != "" {
		domain = ctx.viewDomain
	} else if ctx.route != nil && ctx.route.ViewDomainName != "" {
		domain = ctx.route.ViewDomainName
	}
	body, err := renderer.RenderString(ctx, domain, name, data)
	if err != nil {
		return err
	}
	return ctx.HTML(body)
}

// Asset returns a public asset URL.
func (ctx *Context) Asset(path string, domains ...string) string {
	assets := Service[AssetResolver](ctx)
	if assets == nil {
		return path
	}
	domain := ""
	if len(domains) > 0 {
		domain = domains[0]
	}
	return assets.URL(domain, path)
}

// Blob writes bytes with an explicit content type using the pending status.
func (ctx *Context) Blob(contentType string, body []byte) error {
	return ctx.Type(contentType).Send(body)
}

// SendStream writes a stream response using the pending status.
func (ctx *Context) SendStream(reader io.Reader, contentType ...string) error {
	if len(contentType) > 0 {
		ctx.Type(contentType[0])
	}
	if err := ctx.saveSessions(); err != nil {
		return err
	}
	ctx.ensureContentType("")
	ctx.writer.WriteHeader(ctx.StatusCode(http.StatusOK))
	_, err := io.Copy(ctx.writer, reader)
	return err
}

// SendFile writes a file response.
func (ctx *Context) SendFile(path string) error {
	if err := ctx.saveSessions(); err != nil {
		return err
	}
	if ctx.status > 0 {
		ctx.writer.WriteHeader(ctx.status)
	}
	http.ServeFile(ctx.writer, ctx.request, path)
	return nil
}

// JSON writes a JSON response using the pending status.
func (ctx *Context) JSON(body any) error {
	if ctx.route != nil && !ctx.route.RawResponse && ctx.route.EnvelopeDef != nil {
		wrapped, err := ctx.route.EnvelopeDef.Wrap(ctx, body)
		if err != nil {
			return err
		}
		body = wrapped
	}
	if err := ctx.saveSessions(); err != nil {
		return err
	}
	ctx.ensureContentType(core.MIMEApplicationJSON)
	ctx.writer.WriteHeader(ctx.StatusCode(http.StatusOK))
	return json.NewEncoder(ctx.writer).Encode(body)
}

// XML writes an XML response using the pending status.
func (ctx *Context) XML(body any) error {
	if ctx.route != nil && !ctx.route.RawResponse && ctx.route.EnvelopeDef != nil {
		wrapped, err := ctx.route.EnvelopeDef.Wrap(ctx, body)
		if err != nil {
			return err
		}
		body = wrapped
	}
	if err := ctx.saveSessions(); err != nil {
		return err
	}
	ctx.ensureContentType(core.MIMEApplicationXML)
	ctx.writer.WriteHeader(ctx.StatusCode(http.StatusOK))
	return xml.NewEncoder(ctx.writer).Encode(body)
}

// Redirect redirects the request using the pending status or 302.
func (ctx *Context) Redirect(url string, status ...int) error {
	if err := ctx.saveSessions(); err != nil {
		return err
	}
	code := http.StatusFound
	if len(status) > 0 && status[0] > 0 {
		code = status[0]
	} else if ctx.status > 0 {
		code = ctx.status
	}
	http.Redirect(ctx.writer, ctx.request, url, code)
	return nil
}

func (ctx *Context) write(body []byte, defaultContentType string) error {
	if err := ctx.saveSessions(); err != nil {
		return err
	}
	ctx.ensureContentType(defaultContentType)
	ctx.writer.WriteHeader(ctx.StatusCode(http.StatusOK))
	_, err := ctx.writer.Write(body)
	return err
}

func (ctx *Context) saveSessions() error {
	for _, saver := range ctx.savers {
		if err := saver(ctx.Context()); err != nil {
			return err
		}
	}
	return nil
}

func (ctx *Context) ensureContentType(contentType string) {
	if ctx.writer.Header().Get("Content-Type") != "" || contentType == "" {
		return
	}
	ctx.Type(contentType)
}

func resolveContentType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return ""
	}
	normalized := strings.ToLower(contentType)
	switch normalized {
	case "text", "txt":
		return core.MIMETextPlain
	case "html", "htm":
		return core.MIMETextHTML
	case "json":
		return core.MIMEApplicationJSON
	case "xml":
		return core.MIMEApplicationXML
	case "bin", "binary", "bytes", "octet", "octet-stream":
		return core.MIMEOctetStream
	case "form":
		return core.MIMEFormURLEncoded
	case "multipart":
		return core.MIMEMultipartForm
	}
	if strings.Contains(contentType, "/") {
		return contentType
	}
	extension := contentType
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	if value := mime.TypeByExtension(extension); value != "" {
		return value
	}
	return contentType
}

func firstScope(scopes ...*scope.Scope) *scope.Scope {
	if len(scopes) == 0 {
		return nil
	}
	return scopes[0]
}

func firstMap(items ...core.Map) core.Map {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func pickName(defaultName string, values ...string) string {
	if len(values) > 0 && values[0] != "" {
		return values[0]
	}
	return defaultName
}
