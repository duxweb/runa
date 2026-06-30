package static

import (
	"bytes"
	"net/http"
	"path"
	"strings"

	"github.com/duxweb/runa/route"
)

// Config configures static file middleware.
type Config struct {
	Next  func(*route.Context) bool
	Root  http.FileSystem
	Path  string
	Index bool
}

// New creates static file middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	server := http.FileServer(config.Root)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			requestPath := ctx.Request().URL.Path
			if !matchPrefix(requestPath, config.Path) {
				return next(ctx)
			}
			if !config.Index && strings.HasSuffix(requestPath, "/") {
				return next(ctx)
			}

			request := ctx.Request().Clone(ctx.Request().Context())
			request.URL.Path = stripPrefix(requestPath, config.Path)
			buffer := newResponseBuffer()
			server.ServeHTTP(buffer, request)
			if buffer.Status() == http.StatusNotFound {
				return next(ctx)
			}
			buffer.FlushTo(ctx.Response())
			return nil
		}
	}
}

func firstConfig(configs ...Config) Config {
	config := Config{Root: http.Dir("."), Path: "/"}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		if provided.Root != nil {
			config.Root = provided.Root
		}
		if provided.Path != "" {
			config.Path = cleanPrefix(provided.Path)
		}
		config.Index = provided.Index
	}
	return config
}

func cleanPrefix(value string) string {
	if value == "" || value == "/" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return strings.TrimRight(value, "/")
}

func matchPrefix(requestPath string, prefix string) bool {
	if prefix == "/" {
		return true
	}
	return requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/")
}

func stripPrefix(requestPath string, prefix string) string {
	if prefix == "/" {
		return path.Clean("/" + requestPath)
	}
	trimmed := strings.TrimPrefix(requestPath, prefix)
	if trimmed == "" {
		return "/"
	}
	return path.Clean("/" + trimmed)
}

type responseBuffer struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newResponseBuffer() *responseBuffer {
	return &responseBuffer{header: make(http.Header)}
}

func (buffer *responseBuffer) Header() http.Header {
	return buffer.header
}

func (buffer *responseBuffer) Write(body []byte) (int, error) {
	if buffer.status == 0 {
		buffer.status = http.StatusOK
	}
	return buffer.body.Write(body)
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

func (buffer *responseBuffer) FlushTo(writer http.ResponseWriter) {
	for key, values := range buffer.header {
		for _, value := range values {
			writer.Header().Add(key, value)
		}
	}
	writer.WriteHeader(buffer.Status())
	_, _ = writer.Write(buffer.body.Bytes())
}
