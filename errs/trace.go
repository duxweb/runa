package errs

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// Frame describes one captured error source frame.
type Frame struct {
	File     string
	Line     int
	Function string
}

// String formats the source frame as file:line function.
func (frame Frame) String() string {
	if frame.File == "" {
		return ""
	}
	if frame.Function == "" {
		return fmt.Sprintf("%s:%d", frame.File, frame.Line)
	}
	return fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function)
}

func captureFrames(skip int) []Frame {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return nil
	}
	frames := runtime.CallersFrames(pcs[:n])
	items := make([]Frame, 0, n)
	for {
		frame, more := frames.Next()
		if !skipFrame(frame) {
			items = append(items, Frame{
				File:     frame.File,
				Line:     frame.Line,
				Function: frame.Function,
			})
		}
		if !more {
			break
		}
	}
	return items
}

func skipFrame(frame runtime.Frame) bool {
	function := frame.Function
	if strings.Contains(function, "runtime.Callers") {
		return true
	}
	return strings.Contains(function, "github.com/duxweb/runa/errs.captureFrames")
}

func formatStack(frames []Frame) string {
	if len(frames) == 0 {
		return ""
	}
	var builder strings.Builder
	for index, frame := range frames {
		if index > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(frame.String())
	}
	return builder.String()
}

func trimSource(file string) string {
	if file == "" {
		return ""
	}
	return filepath.ToSlash(file)
}
