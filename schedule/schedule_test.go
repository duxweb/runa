package schedule

import (
	"context"
	"testing"

	"github.com/duxweb/runa/task"
)

type schedulePayload struct {
	ID int `json:"id"`
}

type captureDispatcher struct {
	messages []task.Message
}

func (dispatcher *captureDispatcher) Dispatch(ctx context.Context, message task.Message) (string, error) {
	dispatcher.messages = append(dispatcher.messages, message)
	return "job-1", nil
}

func TestSkipIfQueuedAddsTaskUnique(t *testing.T) {
	tasks := task.New()
	dispatcher := &captureDispatcher{}
	tasks.QueueDispatcher(dispatcher)
	tasks.Register[schedulePayload]("sync", func(context.Context, *task.TaskOf[schedulePayload]) error { return nil })

	registry := New()
	registry.Register("nightly", "1m", "sync", schedulePayload{ID: 1}, Queue("default"), SkipIfQueued())
	unit := NewUnit(registry, tasks)
	unit.run(context.Background(), registry.list()[0])

	if len(dispatcher.messages) != 1 {
		t.Fatalf("messages = %#v", dispatcher.messages)
	}
	message := dispatcher.messages[0]
	if message.Unique != "schedule:nightly" {
		t.Fatalf("unique = %q", message.Unique)
	}
	if message.UniqueStrategy != "until-done" {
		t.Fatalf("unique strategy = %q", message.UniqueStrategy)
	}
}

func TestQueueWithoutSkipIfQueuedDoesNotAddUnique(t *testing.T) {
	tasks := task.New()
	dispatcher := &captureDispatcher{}
	tasks.QueueDispatcher(dispatcher)
	tasks.Register[schedulePayload]("sync", func(context.Context, *task.TaskOf[schedulePayload]) error { return nil })

	registry := New()
	registry.Register("nightly", "1m", "sync", schedulePayload{ID: 1}, Queue("default"))
	unit := NewUnit(registry, tasks)
	unit.run(context.Background(), registry.list()[0])

	if len(dispatcher.messages) != 1 {
		t.Fatalf("messages = %#v", dispatcher.messages)
	}
	if dispatcher.messages[0].Unique != "" {
		t.Fatalf("unique = %q", dispatcher.messages[0].Unique)
	}
}
