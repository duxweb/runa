package audit

import (
	"context"

	"github.com/duxweb/runa/errs"
	"github.com/duxweb/runa/queue"
)

const JobName = "runa.audit"

// QueueWriter pushes audit entries into a Runa queue registry.
func QueueWriter(registry *queue.Registry, queueName string, options ...queue.PushOption) Writer {
	return FuncWriter(func(ctx context.Context, entry Entry) error {
		if registry == nil {
			return errs.New("audit queue registry is nil")
		}
		if queueName == "" {
			queueName = "audit"
		}
		_, err := registry.Push(ctx, queueName, JobName, entry, options...)
		return err
	})
}

// DefaultQueueWriter pushes audit entries into the default queue registry.
func DefaultQueueWriter(queueName string, options ...queue.PushOption) Writer {
	return FuncWriter(func(ctx context.Context, entry Entry) error {
		return QueueWriter(queue.Default(), queueName, options...).Write(ctx, entry)
	})
}

// HandleQueue registers the audit queue handler.
func HandleQueue(registry *queue.Registry, handler func(context.Context, Entry) error, options ...queue.JobOption) {
	if registry == nil || handler == nil {
		return
	}
	registry.Job[Entry](JobName, func(ctx context.Context, job *queue.Job[Entry]) error {
		return handler(ctx, job.Payload)
	}, options...)
}

// HandleDefaultQueue registers the audit queue handler on the default queue registry.
func HandleDefaultQueue(handler func(context.Context, Entry) error, options ...queue.JobOption) {
	HandleQueue(queue.Default(), handler, options...)
}
