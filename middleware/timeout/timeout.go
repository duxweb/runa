package timeout

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
)

// Config configures timeout middleware.
type Config struct {
	Next    func(*route.Context) bool
	Timeout time.Duration
	Message string
}

// New creates timeout middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			if isUpgrade(ctx.Request()) {
				return next(ctx)
			}
			requestCtx, cancel := context.WithTimeout(ctx.Request().Context(), config.Timeout)
			defer cancel()
			request := ctx.Request().WithContext(requestCtx)
			ctx.SetRequest(request)

			original := ctx.Response()
			buffer := newTimeoutWriter(original)
			ctx.SetResponse(buffer)
			done := make(chan error, 1)
			go func() {
				defer func() {
					if value := recover(); value != nil {
						done <- route.PanicError(value)
					}
				}()
				done <- next(ctx)
			}()

			select {
			case err := <-done:
				ctx.SetResponse(original)
				if err != nil && !buffer.Written() {
					return err
				}
				if !buffer.Flush() {
					return nil
				}
				return err
			case <-requestCtx.Done():
				buffer.Timeout(config.Message)
				return nil
			}
		}
	}
}

func firstConfig(configs ...Config) Config {
	config := Config{Timeout: 30 * time.Second, Message: "request timeout"}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		if provided.Timeout > 0 {
			config.Timeout = provided.Timeout
		}
		if provided.Message != "" {
			config.Message = provided.Message
		}
	}
	return config
}

func isUpgrade(request *http.Request) bool {
	if request == nil {
		return false
	}
	if strings.Contains(strings.ToLower(request.Header.Get("Connection")), "upgrade") {
		return true
	}
	return request.Header.Get("Upgrade") != ""
}

type timeoutWriter struct {
	writer  http.ResponseWriter
	header  http.Header
	status  int
	closed  bool
	written bool
	mu      sync.Mutex
}

func newTimeoutWriter(writer http.ResponseWriter) *timeoutWriter {
	return &timeoutWriter{
		writer: writer,
		header: make(http.Header),
	}
}

func (writer *timeoutWriter) Header() http.Header {
	return writer.header
}

func (writer *timeoutWriter) Write(body []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.closed {
		return len(body), nil
	}
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	if !writer.written {
		writer.writeHeaderLocked(writer.status)
	}
	return writer.writer.Write(body)
}

func (writer *timeoutWriter) WriteHeader(status int) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.closed || writer.written {
		return
	}
	writer.status = status
	writer.writeHeaderLocked(status)
}

func (writer *timeoutWriter) Flush() bool {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.closed {
		return false
	}
	writer.closed = true
	if !writer.written {
		status := writer.status
		if status == 0 {
			status = http.StatusOK
		}
		writer.writeHeaderLocked(status)
	}
	return true
}

func (writer *timeoutWriter) Written() bool {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.written
}

func (writer *timeoutWriter) Timeout(message string) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.closed {
		return
	}
	writer.closed = true
	if writer.written {
		return
	}
	writer.writer.Header().Set("Content-Type", core.MIMETextPlain)
	writer.writer.WriteHeader(http.StatusRequestTimeout)
	_, _ = writer.writer.Write([]byte(message))
	writer.written = true
}

func (writer *timeoutWriter) Close() {
	writer.mu.Lock()
	writer.closed = true
	writer.mu.Unlock()
}

func (writer *timeoutWriter) writeHeaderLocked(status int) {
	for key, values := range writer.header {
		for _, value := range values {
			writer.writer.Header().Add(key, value)
		}
	}
	writer.writer.WriteHeader(status)
	writer.written = true
}
