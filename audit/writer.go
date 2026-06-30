package audit

import (
	"context"
	"errors"
	"log/slog"

	runlog "github.com/duxweb/runa/log"
)

// Writer writes audit entries.
type Writer interface {
	Write(ctx context.Context, entry Entry) error
}

// FuncWriter adapts a function to Writer.
type FuncWriter func(ctx context.Context, entry Entry) error

// Write writes an audit entry.
func (fn FuncWriter) Write(ctx context.Context, entry Entry) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, entry)
}

type noopWriter struct{}

// NoopWriter returns a writer that drops entries.
func NoopWriter() Writer { return noopWriter{} }

func (noopWriter) Write(context.Context, Entry) error { return nil }

// MultiWriter combines multiple writers.
func MultiWriter(writers ...Writer) Writer { return multiWriter{writers: writers} }

type multiWriter struct{ writers []Writer }

func (writer multiWriter) Write(ctx context.Context, entry Entry) error {
	var joined error
	for _, item := range writer.writers {
		if item == nil {
			continue
		}
		if err := item.Write(ctx, entry); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// LogWriter writes audit records to a Runa logger registry.
func LogWriter(registry *runlog.Registry, channels ...string) Writer {
	channel := "audit"
	if len(channels) > 0 && channels[0] != "" {
		channel = channels[0]
	}
	return FuncWriter(func(ctx context.Context, entry Entry) error {
		logger := slog.Default()
		if registry != nil {
			logger = registry.Get(channel)
		}
		logger.InfoContext(ctx, "audit",
			slog.String("route", entry.Route),
			slog.String("action", entry.Action),
			slog.String("method", entry.Method),
			slog.String("path", entry.Path),
			slog.Int("status", entry.Status),
			slog.Bool("success", entry.Success),
			slog.String("actor_id", entry.ActorID),
			slog.String("request_id", entry.RequestID),
		)
		return nil
	})
}

// DefaultLogWriter writes audit records to the default Runa logger registry.
func DefaultLogWriter(channels ...string) Writer {
	return FuncWriter(func(ctx context.Context, entry Entry) error {
		return LogWriter(runlog.Default(), channels...).Write(ctx, entry)
	})
}
