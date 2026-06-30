package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/duxweb/runa/audit"
	"github.com/duxweb/runa/auth"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
)

// Option configures audit HTTP middleware.
type Option func(*options)

type options struct {
	next  func(ctx *route.Context) bool
	build func(ctx *route.Context, entry *audit.Entry) error
}

// Next skips audit for matching requests.
func Next(fn func(ctx *route.Context) bool) Option {
	return func(options *options) { options.next = fn }
}

// Build customizes the audit entry before writing.
func Build(fn func(ctx *route.Context, entry *audit.Entry) error) Option {
	return func(options *options) { options.build = fn }
}

// New creates audit middleware.
func New(config audit.Config, items ...Option) route.Middleware {
	config = audit.Normalize(config)
	current := options{}
	for _, item := range items {
		if item != nil {
			item(&current)
		}
	}
	methods := methodSet(config.Methods)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if _, ok := methods[strings.ToUpper(ctx.Request().Method)]; !ok {
				return next(ctx)
			}
			if current.next != nil && current.next(ctx) {
				return next(ctx)
			}

			started := time.Now()
			input := core.Map(nil)
			if config.CaptureInput {
				input = captureInput(ctx, config)
			}
			original := ctx.Response()
			recorder := route.NewStatusRecorder(original)
			var buffer *responseBuffer
			if config.Strict {
				buffer = newResponseBuffer(original)
				ctx.SetResponse(buffer)
			} else {
				ctx.SetResponse(recorder)
			}
			err := next(ctx)
			if buffer != nil {
				recorder = route.NewStatusRecorder(buffer)
				recorder.WriteHeader(buffer.Status())
			}

			entry := buildEntry(ctx, recorder, err, input, started)
			if current.build != nil {
				if buildErr := current.build(ctx, &entry); buildErr != nil && err == nil {
					err = buildErr
					entry.Error = buildErr.Error()
					entry.Success = false
				}
			}
			if writeErr := audit.Write(ctx.Context(), config, entry); writeErr != nil && config.Strict && err == nil {
				ctx.SetResponse(original)
				return writeErr
			}
			if buffer != nil {
				ctx.SetResponse(original)
				if err == nil {
					if flushErr := buffer.Flush(); flushErr != nil {
						return flushErr
					}
				}
			}
			return err
		}
	}
}

// Use creates middleware from an audit registry.
func Use(registry *audit.Registry, items ...Option) route.Middleware {
	return New(registry.Config(), items...)
}

// Default creates middleware from the default audit registry.
func Default(items ...Option) route.Middleware {
	return Use(audit.Default(), items...)
}

func buildEntry(ctx *route.Context, recorder *route.StatusRecorder, err error, input core.Map, started time.Time) audit.Entry {
	routeName := ""
	action := ""
	meta := core.Map(nil)
	if current := ctx.Route(); current != nil {
		routeName = current.RouteID()
		action = route.MetaAs[string](current, "action")
		if action == "" {
			action = routeName
		}
		if len(current.MetaData) > 0 {
			meta = make(core.Map, len(current.MetaData))
			for key, value := range current.MetaData {
				meta[key] = value
			}
		}
	}
	status := recorder.Status()
	if err != nil && status < http.StatusBadRequest {
		status = route.ErrorStatus(err)
	}
	entry := audit.Entry{
		Time:      core.Now(),
		Method:    ctx.Request().Method,
		Path:      ctx.Request().URL.Path,
		Route:     routeName,
		Action:    action,
		Status:    status,
		Success:   err == nil && status < http.StatusBadRequest,
		Duration:  time.Since(started),
		IP:        ctx.IP(),
		UserAgent: route.Header[string](ctx, "User-Agent"),
		RequestID: ctx.RequestID(),
		TraceID:   route.Header[string](ctx, "Traceparent"),
		Input:     input,
		Meta:      meta,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	if info := authInfo(ctx); info != nil {
		entry.Guard = info.Name
		entry.ActorID = core.Cast[string](info.Data["id"])
		if entry.ActorID == "" {
			entry.ActorID = core.Cast[string](info.Data["user_id"])
		}
		entry.ActorName = core.Cast[string](info.Data["name"])
		if entry.ActorName == "" {
			entry.ActorName = core.Cast[string](info.Data["username"])
		}
	}
	return entry
}

func authInfo(ctx *route.Context) *auth.Info {
	info, _ := ctx.Locals("runa.auth").(*auth.Info)
	return info
}

func captureInput(ctx *route.Context, config audit.Config) core.Map {
	input := core.Map{}
	for key, values := range ctx.Request().URL.Query() {
		if len(values) == 1 {
			input[key] = values[0]
		} else if len(values) > 1 {
			input[key] = values
		}
	}
	if body := captureBody(ctx, config.MaxInputSize); len(body) > 0 {
		var jsonBody map[string]any
		contentType := ctx.Request().Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") && json.Unmarshal(body, &jsonBody) == nil {
			for key, value := range jsonBody {
				input[key] = value
			}
		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			values, err := url.ParseQuery(string(body))
			if err == nil {
				for key, items := range values {
					if len(items) == 1 {
						input[key] = items[0]
					} else if len(items) > 1 {
						input[key] = items
					}
				}
			} else {
				input["body"] = string(body)
			}
		} else {
			input["body"] = string(body)
		}
	}
	return audit.Mask(input, config.MaskFields, config.MaskValue)
}

func captureBody(ctx *route.Context, limit int) []byte {
	request := ctx.Request()
	if request == nil || request.Body == nil || limit <= 0 {
		return nil
	}
	if strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/") {
		return nil
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	ctx.SetRequest(request)
	if len(body) > limit {
		return body[:limit]
	}
	return body
}

func methodSet(methods []string) map[string]struct{} {
	set := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method != "" {
			set[method] = struct{}{}
		}
	}
	return set
}

type responseBuffer struct {
	writer http.ResponseWriter
	header http.Header
	status int
	body   []byte
}

func newResponseBuffer(writer http.ResponseWriter) *responseBuffer {
	return &responseBuffer{writer: writer, header: make(http.Header)}
}

func (buffer *responseBuffer) Header() http.Header { return buffer.header }

func (buffer *responseBuffer) Write(body []byte) (int, error) {
	if buffer.status == 0 {
		buffer.status = http.StatusOK
	}
	buffer.body = append(buffer.body, body...)
	return len(body), nil
}

func (buffer *responseBuffer) WriteHeader(status int) {
	if buffer.status != 0 {
		return
	}
	buffer.status = status
}

func (buffer *responseBuffer) Status() int {
	if buffer.status == 0 {
		return http.StatusOK
	}
	return buffer.status
}

func (buffer *responseBuffer) Flush() error {
	for key, values := range buffer.header {
		for _, value := range values {
			buffer.writer.Header().Add(key, value)
		}
	}
	buffer.writer.WriteHeader(buffer.Status())
	if len(buffer.body) == 0 {
		return nil
	}
	_, err := buffer.writer.Write(buffer.body)
	return err
}
