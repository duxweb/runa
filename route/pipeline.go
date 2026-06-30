package route

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

func (route *Route) errorPipeline(app ErrorPipeline) ErrorPipeline {
	pipeline := app
	if route.Errors.OnError != nil {
		pipeline.OnError = route.Errors.OnError
	}
	if route.Errors.Renderer != nil {
		pipeline.Renderer = route.Errors.Renderer
	}
	return pipeline
}

type panicError struct {
	value  any
	stack  []byte
	frames []runtime.Frame
}

func newPanicError(value any, capture ...bool) panicError {
	if len(capture) > 0 && !capture[0] {
		return panicError{value: value}
	}
	stack := debug.Stack()
	return panicError{value: value, stack: stack, frames: panicFrames(stack)}
}

func (err panicError) Error() string {
	return fmt.Sprintf("panic: %v", err.value)
}

func (err panicError) ErrorStatus() int     { return 500 }
func (err panicError) ErrorMessage() string { return "Internal Server Error" }
func (err panicError) Stack() []byte        { return append([]byte(nil), err.stack...) }
func (err panicError) Source() string {
	if len(err.frames) == 0 {
		return ""
	}
	return sourceString(err.frames[0])
}

type notFoundError struct{}

func (notFoundError) Error() string        { return "Not Found" }
func (notFoundError) ErrorStatus() int     { return 404 }
func (notFoundError) ErrorMessage() string { return "Not Found" }

type methodNotAllowedError struct{}

func (methodNotAllowedError) Error() string        { return "Method Not Allowed" }
func (methodNotAllowedError) ErrorStatus() int     { return 405 }
func (methodNotAllowedError) ErrorMessage() string { return "Method Not Allowed" }

func formatFrames(frames []runtime.Frame) string {
	if len(frames) == 0 {
		return ""
	}
	var builder strings.Builder
	for index, frame := range frames {
		if index > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(sourceString(frame))
	}
	return builder.String()
}

func sourceString(frame runtime.Frame) string {
	if frame.File == "" {
		return ""
	}
	if frame.Function == "" {
		return fmt.Sprintf("%s:%d", frame.File, frame.Line)
	}
	return fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function)
}

func panicFrames(stack []byte) []runtime.Frame {
	lines := strings.Split(string(stack), "\n")
	frames := make([]runtime.Frame, 0)
	for index := 0; index+1 < len(lines); index += 2 {
		function := strings.TrimSpace(lines[index])
		if function == "" {
			continue
		}
		if strings.HasPrefix(function, "goroutine ") {
			index--
			continue
		}
		file, line := parseStackLocation(strings.TrimSpace(lines[index+1]))
		if file == "" || skipPanicFrame(function) {
			continue
		}
		frames = append(frames, runtime.Frame{File: file, Line: line, Function: function})
	}
	return frames
}

func parseStackLocation(value string) (string, int) {
	if value == "" {
		return "", 0
	}
	if tab := strings.Index(value, "\t"); tab >= 0 {
		value = value[tab+1:]
	}
	if plus := strings.LastIndex(value, " +"); plus >= 0 {
		value = value[:plus]
	}
	colon := strings.LastIndex(value, ":")
	if colon < 0 {
		return value, 0
	}
	line, _ := strconv.Atoi(value[colon+1:])
	return value[:colon], line
}

func skipPanicFrame(function string) bool {
	switch {
	case strings.HasPrefix(function, "runtime/"),
		strings.HasPrefix(function, "runtime."),
		strings.HasPrefix(function, "panic("),
		strings.HasPrefix(function, "runtime/debug."),
		strings.Contains(function, "github.com/duxweb/runa/route.newPanicError"),
		strings.Contains(function, "github.com/duxweb/runa/middleware/recover."):
		return true
	default:
		return false
	}
}
