package audit

import (
	"context"

	"github.com/duxweb/runa/core"
)

// Record writes one manual audit entry.
func Record(ctx context.Context, writer Writer, entry Entry) error {
	if writer == nil {
		return nil
	}
	if entry.Time.IsZero() {
		entry.Time = core.Now()
	}
	return writer.Write(ctx, entry)
}

// Write writes an audit entry using config writer and mode.
func Write(ctx context.Context, config Config, entry Entry) error {
	config = Normalize(config)
	writer := config.Writer
	if writer == nil && config.Write != nil {
		writer = FuncWriter(config.Write)
	}
	if writer == nil {
		writer = NoopWriter()
	}
	base := context.Background()
	if config.Mode == Sync || config.Strict {
		base = ctx
	}
	writeCtx := base
	cancel := func() {}
	if config.WriteTimeout > 0 {
		writeCtx, cancel = context.WithTimeout(base, config.WriteTimeout)
	}
	call := func() error {
		defer cancel()
		return writer.Write(writeCtx, entry)
	}
	if config.Mode == Sync || config.Strict {
		return call()
	}
	go func() { _ = call() }()
	return nil
}
