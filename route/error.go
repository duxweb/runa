package route

import (
	"encoding/xml"
	"errors"
	"log/slog"
	"net/http"
	"runtime"
	"strings"

	"github.com/duxweb/runa/core"
)

const errorLogChannel = "error"

// ErrorHandler normalizes route errors.
type ErrorHandler func(*Context, error) error

// ErrorRenderer renders route errors.
type ErrorRenderer interface {
	RenderError(*Context, error) error
}

// ErrorRendererFunc adapts a function to ErrorRenderer.
type ErrorRendererFunc func(*Context, error) error

// RenderError renders an error.
func (fn ErrorRendererFunc) RenderError(ctx *Context, err error) error {
	return fn(ctx, err)
}

// HTTPError is a route-layer HTTP error.
type HTTPError struct {
	Status  int
	Code    string
	Message string
	Params  core.Map
	Cause   error
	frames  []runtime.Frame
}

// ErrorStatus returns the HTTP status represented by err.
func ErrorStatus(err error) int { return errorStatus(err) }

// ErrorCode returns a stable error code represented by err.
func ErrorCode(err error) string { return errorCode(err) }

// ErrorMessage returns the public error message represented by err.
func ErrorMessage(err error) string { return errorMessage(err) }

// ErrorParams returns translation/rendering parameters represented by err.
func ErrorParams(err error) core.Map { return errorParams(err) }

// PanicError converts a recovered panic value into a route error.
func PanicError(value any, stack ...bool) error { return newPanicError(value, stack...) }

// Error returns the error message.
func (err *HTTPError) Error() string {
	if err == nil {
		return ""
	}
	if err.Message != "" {
		return err.Message
	}
	return http.StatusText(err.Status)
}

// Unwrap returns the cause error.
func (err *HTTPError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// ErrorStatus returns the HTTP status.
func (err *HTTPError) ErrorStatus() int {
	if err == nil || err.Status == 0 {
		return http.StatusInternalServerError
	}
	return err.Status
}

// ErrorCode returns the standard error code.
func (err *HTTPError) ErrorCode() string {
	if err == nil || err.Code == "" {
		return statusCodeName(err.ErrorStatus())
	}
	return err.Code
}

// ErrorMessage returns the public error message.
func (err *HTTPError) ErrorMessage() string {
	return err.Error()
}

// ErrorParams returns error parameters.
func (err *HTTPError) ErrorParams() core.Map {
	if err == nil {
		return nil
	}
	return err.Params
}

// Source returns the top source frame captured when this HTTP error was created.
func (err *HTTPError) Source() string {
	if err == nil || len(err.frames) == 0 {
		return ""
	}
	frame := err.frames[0]
	return sourceString(frame)
}

// Stack returns captured source frames.
func (err *HTTPError) Stack() []runtime.Frame {
	if err == nil || len(err.frames) == 0 {
		return nil
	}
	return append([]runtime.Frame(nil), err.frames...)
}

// Envelope wraps successful response data.
type Envelope interface {
	Wrap(*Context, any) (any, error)
}

// EnvelopeFunc adapts a function to Envelope.
type EnvelopeFunc func(*Context, any) (any, error)

// Wrap wraps response data.
func (fn EnvelopeFunc) Wrap(ctx *Context, data any) (any, error) {
	return fn(ctx, data)
}

// ErrorPipeline handles route errors.
type ErrorPipeline struct {
	OnError  ErrorHandler
	Renderer ErrorRenderer
}

// Handle normalizes and renders an error.
func (pipeline ErrorPipeline) Handle(ctx *Context, err error) error {
	if err == nil {
		return nil
	}
	if pipeline.OnError != nil {
		err = pipeline.OnError(ctx, err)
	}
	if err == nil {
		return nil
	}
	logServerError(ctx, err)
	renderer := pipeline.Renderer
	if renderer == nil {
		renderer = DefaultErrorRenderer{}
	}
	return renderer.RenderError(ctx, err)
}

func logServerError(ctx *Context, err error) {
	if errorStatus(err) < http.StatusInternalServerError || ctx == nil {
		return
	}
	loggers := Service[LoggerProvider](ctx)
	if loggers == nil {
		return
	}
	attrs := []slog.Attr{
		slog.Int("status", errorStatus(err)),
		slog.String("code", errorCode(err)),
	}
	if request := ctx.Request(); request != nil {
		attrs = append(attrs,
			slog.String("method", request.Method),
			slog.String("path", request.URL.Path),
		)
	}
	attrs = appendErrorTrace(attrs, err)
	loggers.Get(errorLogChannel).LogAttrs(ctx.Context(), slog.LevelError, err.Error(), attrs...)
}

func appendErrorTrace(attrs []slog.Attr, err error) []slog.Attr {
	if source := errorSource(err); source != "" {
		attrs = append(attrs, slog.String("source", source))
	}
	if stack := errorStack(err); stack != "" {
		attrs = append(attrs, slog.String("stack", stack))
	}
	return attrs
}

func errorSource(err error) string {
	var source interface{ Source() string }
	if errors.As(err, &source) && source.Source() != "" {
		return source.Source()
	}
	var oopsSource interface{ Sources() string }
	if errors.As(err, &oopsSource) && oopsSource.Sources() != "" {
		return firstLine(oopsSource.Sources())
	}
	return ""
}

func errorStack(err error) string {
	var textStack interface{ StackTrace() string }
	if errors.As(err, &textStack) && textStack.StackTrace() != "" {
		return textStack.StackTrace()
	}
	var byteStack interface{ Stack() []byte }
	if errors.As(err, &byteStack) && len(byteStack.Stack()) > 0 {
		return string(byteStack.Stack())
	}
	var frameStack interface{ Stack() []runtime.Frame }
	if errors.As(err, &frameStack) {
		return formatFrames(frameStack.Stack())
	}
	var oopsStack interface{ Stacktrace() string }
	if errors.As(err, &oopsStack) && oopsStack.Stacktrace() != "" {
		return oopsStack.Stacktrace()
	}
	return ""
}

func firstLine(value string) string {
	for index, char := range value {
		if char == '\n' || char == '\r' {
			return value[:index]
		}
	}
	return value
}

// DefaultErrorRenderer renders errors as plain text.
type DefaultErrorRenderer struct{}

// RenderError renders a plain text error.
func (DefaultErrorRenderer) RenderError(ctx *Context, err error) error {
	status := errorStatus(err)
	message := errorMessage(err)
	return ctx.Status(status).Text(ctx.T(message, errorParams(err)))
}

// TextErrorRenderer renders plain text errors.
type TextErrorRenderer struct{}

// RenderError renders a plain text error.
func (TextErrorRenderer) RenderError(ctx *Context, err error) error {
	return DefaultErrorRenderer{}.RenderError(ctx, err)
}

// JSONErrorRenderer renders JSON errors.
type JSONErrorRenderer struct{}

// RenderError renders a JSON error.
func (JSONErrorRenderer) RenderError(ctx *Context, err error) error {
	payload := core.Map{
		"code":    errorCode(err),
		"message": ctx.T(errorMessage(err), errorParams(err)),
	}
	if validation := validationError(err); validation != nil {
		payload["errors"] = validation
	}
	return ctx.Status(errorStatus(err)).JSON(payload)
}

// XMLErrorRenderer renders XML errors.
type XMLErrorRenderer struct{}

type xmlError struct {
	XMLName xml.Name `xml:"error"`
	Code    string   `xml:"code"`
	Message string   `xml:"message"`
}

// RenderError renders an XML error.
func (XMLErrorRenderer) RenderError(ctx *Context, err error) error {
	return ctx.Status(errorStatus(err)).XML(xmlError{Code: errorCode(err), Message: ctx.T(errorMessage(err), errorParams(err))})
}

// HTMLErrorRenderer renders HTML errors.
type HTMLErrorRenderer struct{}

// RenderError renders an HTML error.
func (HTMLErrorRenderer) RenderError(ctx *Context, err error) error {
	return ctx.Status(errorStatus(err)).HTML("<h1>" + htmlEscape(ctx.T(errorMessage(err), errorParams(err))) + "</h1>")
}

// NegotiatedErrorRenderer renders errors based on Accept header.
type NegotiatedErrorRenderer struct{}

// RenderError renders an error based on Accept header.
func (NegotiatedErrorRenderer) RenderError(ctx *Context, err error) error {
	accept := Header[string](ctx, "Accept")
	switch {
	case strings.Contains(accept, "application/json"):
		return JSONErrorRenderer{}.RenderError(ctx, err)
	case strings.Contains(accept, "application/xml"), strings.Contains(accept, "text/xml"):
		return XMLErrorRenderer{}.RenderError(ctx, err)
	case strings.Contains(accept, "text/html"):
		return HTMLErrorRenderer{}.RenderError(ctx, err)
	default:
		return TextErrorRenderer{}.RenderError(ctx, err)
	}
}

func validationError(err error) any {
	var validation interface{ ErrorFields() any }
	if errors.As(err, &validation) {
		return validation.ErrorFields()
	}
	return nil
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")
	return replacer.Replace(value)
}

func errorStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var status interface{ ErrorStatus() int }
	if errors.As(err, &status) && status.ErrorStatus() > 0 {
		return status.ErrorStatus()
	}
	return http.StatusInternalServerError
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	var message interface{ ErrorMessage() string }
	if errors.As(err, &message) && message.ErrorMessage() != "" {
		return message.ErrorMessage()
	}
	status := errorStatus(err)
	if status >= 500 {
		return http.StatusText(status)
	}
	return err.Error()
}

func errorCode(err error) string {
	if err == nil {
		return ""
	}
	var code interface{ ErrorCode() string }
	if errors.As(err, &code) && code.ErrorCode() != "" {
		return code.ErrorCode()
	}
	return statusCodeName(errorStatus(err))
}

func errorParams(err error) core.Map {
	if err == nil {
		return nil
	}
	var params interface{ ErrorParams() core.Map }
	if errors.As(err, &params) {
		return params.ErrorParams()
	}
	return nil
}

func statusCodeName(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusInternalServerError:
		return "internal"
	default:
		return http.StatusText(status)
	}
}
