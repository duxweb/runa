package route

import "net/http"

// SetResponse replaces the response writer used by this context.
func (ctx *Context) SetResponse(writer http.ResponseWriter) {
	ctx.writer = writer
}

// StatusRecorder records response status while proxying writes.
type StatusRecorder struct {
	writer http.ResponseWriter
	status int
}

// NewStatusRecorder creates a response status recorder.
func NewStatusRecorder(writer http.ResponseWriter) *StatusRecorder {
	return &StatusRecorder{writer: writer}
}

// Header returns the proxied response headers.
func (recorder *StatusRecorder) Header() http.Header {
	return recorder.writer.Header()
}

// Write writes response body.
func (recorder *StatusRecorder) Write(body []byte) (int, error) {
	if recorder.status == 0 {
		recorder.WriteHeader(http.StatusOK)
	}
	return recorder.writer.Write(body)
}

// WriteHeader writes and records status code.
func (recorder *StatusRecorder) WriteHeader(status int) {
	if recorder.status != 0 {
		return
	}
	recorder.status = status
	recorder.writer.WriteHeader(status)
}

// Status returns the recorded status code.
func (recorder *StatusRecorder) Status() int {
	if recorder.status == 0 {
		return http.StatusOK
	}
	return recorder.status
}

// Written reports whether a status has been written.
func (recorder *StatusRecorder) Written() bool {
	return recorder.status != 0
}

// Unwrap returns the underlying response writer.
func (recorder *StatusRecorder) Unwrap() http.ResponseWriter {
	return recorder.writer
}
