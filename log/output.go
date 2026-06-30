package log

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/duxweb/runa/core"
)

// Output builds a slog handler for a channel.
type Output interface {
	Build(ctx context.Context, name string) (slog.Handler, error)
}

type closeableOutput interface {
	Output
	Close(context.Context) error
}

// OutputFunc adapts a function to Output.
type OutputFunc func(context.Context, string) (slog.Handler, error)

// Build builds a slog handler.
func (fn OutputFunc) Build(ctx context.Context, name string) (slog.Handler, error) {
	return fn(ctx, name)
}

// Writer returns a writer output.
func Writer(writer io.Writer, options ...Option) Output {
	return OutputFunc(func(context.Context, string) (slog.Handler, error) {
		return buildHandler(writer, applyOptions(options)), nil
	})
}

// Console returns a stdout output.
func Console(options ...Option) Output {
	items := append([]Option{Pretty()}, options...)
	return Writer(os.Stdout, items...)
}

// File returns a plain file output.
func File(path string, options ...Option) Output {
	return newFileOutput(path, options...)
}

// Handler wraps an existing slog handler.
func Handler(handler slog.Handler) Output {
	return OutputFunc(func(context.Context, string) (slog.Handler, error) {
		return handler, nil
	})
}

// Discard returns a discard output.
func Discard() Output {
	return Writer(io.Discard)
}

type writerOutput struct {
	writer  io.Writer
	options []Option
}

type fileOutput struct {
	path    string
	options []Option
	files   []*os.File
}

func newWriterOutput(writer io.Writer, options ...Option) Output {
	return writerOutput{writer: writer, options: append([]Option(nil), options...)}
}

func newFileOutput(path string, options ...Option) Output {
	return &fileOutput{path: path, options: append([]Option(nil), options...)}
}

func (output writerOutput) Build(context.Context, string) (slog.Handler, error) {
	return buildHandler(output.writer, applyOptions(output.options)), nil
}

func (output writerOutput) Close(context.Context) error {
	if closer, ok := output.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (output *fileOutput) Build(context.Context, string) (slog.Handler, error) {
	file, err := os.OpenFile(output.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	output.files = append(output.files, file)
	return buildHandler(file, applyOptions(output.options)), nil
}

func (output *fileOutput) Close(context.Context) error {
	var err error
	for _, file := range output.files {
		err = errors.Join(err, file.Close())
	}
	output.files = nil
	return err
}

func buildHandler(writer io.Writer, options Options) slog.Handler {
	var handler slog.Handler
	switch options.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, handlerOptions(options))
	case "pretty":
		handler = slog.NewTextHandler(writer, handlerOptions(options))
	default:
		handler = slog.NewTextHandler(writer, handlerOptions(options))
	}
	if len(options.Attrs) > 0 {
		handler = handler.WithAttrs(options.Attrs)
	}
	return handler
}

func prettyAttr(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return attr
	}
	switch attr.Key {
	case slog.TimeKey:
		if attr.Value.Kind() == slog.KindTime {
			return slog.String(attr.Key, core.In(attr.Value.Time()).Format("03:04PM"))
		}
	case slog.LevelKey:
		if level, ok := attr.Value.Any().(slog.Level); ok {
			return slog.String(attr.Key, levelName(level))
		}
	case slog.MessageKey:
		return attr
	}
	return attr
}

func levelName(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERR"
	case level >= slog.LevelWarn:
		return "WRN"
	case level <= slog.LevelDebug:
		return "DBG"
	default:
		return "INF"
	}
}

func fanout(handlers ...slog.Handler) slog.Handler {
	return Fanout(handlers...)
}

// Fanout sends records to multiple handlers.
func Fanout(handlers ...slog.Handler) slog.Handler {
	switch len(handlers) {
	case 0:
		return slog.NewTextHandler(io.Discard, nil)
	case 1:
		return handlers[0]
	default:
		return fanoutHandler(handlers)
	}
}

type fanoutHandler []slog.Handler

func (handler fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, item := range handler {
		if item.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (handler fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	var err error
	for _, item := range handler {
		if item.Enabled(ctx, record.Level) {
			err = errors.Join(err, item.Handle(ctx, record.Clone()))
		}
	}
	return err
}

func (handler fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	items := make([]slog.Handler, 0, len(handler))
	for _, item := range handler {
		items = append(items, item.WithAttrs(attrs))
	}
	return fanoutHandler(items)
}

func (handler fanoutHandler) WithGroup(name string) slog.Handler {
	items := make([]slog.Handler, 0, len(handler))
	for _, item := range handler {
		items = append(items, item.WithGroup(name))
	}
	return fanoutHandler(items)
}
