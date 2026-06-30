package log

import (
	"log/slog"

	"github.com/duxweb/runa/core"
)

// Options configures a slog handler.
type Options struct {
	Level   slog.Leveler
	Format  string
	Source  bool
	Attrs   []slog.Attr
	NoColor bool
}

// Option configures log output options.
type Option func(*Options)

// Level sets minimum log level.
func Level(level slog.Leveler) Option {
	return func(options *Options) { options.Level = level }
}

// Text uses text log format.
func Text() Option {
	return func(options *Options) { options.Format = "text" }
}

// Pretty uses colored console log format.
func Pretty() Option {
	return func(options *Options) { options.Format = "pretty" }
}

// JSON uses JSON log format.
func JSON() Option {
	return func(options *Options) { options.Format = "json" }
}

// Source enables source location.
func Source(enabled bool) Option {
	return func(options *Options) { options.Source = enabled }
}

// Color enables or disables colors for pretty output.
func Color(enabled bool) Option {
	return func(options *Options) { options.NoColor = !enabled }
}

// Attr appends a static handler attr.
func Attr(key string, value any) Option {
	return func(options *Options) { options.Attrs = append(options.Attrs, slog.Any(key, value)) }
}

// BuildOptions resolves log output options.
func BuildOptions(items ...Option) Options {
	options := Options{Level: slog.LevelInfo, Format: "text"}
	for _, item := range items {
		if item != nil {
			item(&options)
		}
	}
	return options
}

func applyOptions(items []Option) Options {
	return BuildOptions(items...)
}

func handlerOptions(options Options) *slog.HandlerOptions {
	handler := &slog.HandlerOptions{Level: options.Level, AddSource: options.Source, ReplaceAttr: timeAttr}
	if options.Format == "pretty" {
		handler.ReplaceAttr = prettyAttr
	}
	return handler
}

func timeAttr(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) > 0 || attr.Key != slog.TimeKey || attr.Value.Kind() != slog.KindTime {
		return attr
	}
	return slog.Time(attr.Key, core.In(attr.Value.Time()))
}
