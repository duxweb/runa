package schedule

import (
	"context"
	"testing"
	"time"

	"github.com/duxweb/runa/core"
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

func TestInfoNextRunUsesCronSpec(t *testing.T) {
	registry := New()
	registry.Register("nightly", "0 2 * * *", "sync", schedulePayload{ID: 1}, Timezone("UTC"))
	info := registry.Info()[0]
	now := time.Date(2026, 7, 4, 1, 30, 0, 0, time.UTC)
	next := info.NextRunAt(now)
	want := time.Date(2026, 7, 4, 2, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next = %s want %s", next, want)
	}
}

func TestInfoNextRunUsesDurationSpec(t *testing.T) {
	info := Info{Spec: "15m", Enabled: true}
	now := time.Date(2026, 7, 4, 1, 30, 0, 0, time.UTC)
	next := info.NextRunAt(now)
	want := now.Add(15 * time.Minute)
	if !next.Equal(want) {
		t.Fatalf("next = %s want %s", next, want)
	}
}

func TestInfoNextRunUsesExplicitTimezone(t *testing.T) {
	info := Info{Spec: "0 2 * * *", Timezone: "Asia/Shanghai", Enabled: true}
	now := time.Date(2026, 7, 3, 17, 30, 0, 0, time.UTC)
	next := info.NextRunAt(now)
	want := time.Date(2026, 7, 3, 18, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next = %s want %s", next, want)
	}
}

func TestInfoNextRunUsesApplicationTimezone(t *testing.T) {
	previousLocation := core.Location()
	defer core.SetLocation(previousLocation)
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	core.SetLocation(location)
	info := Info{Spec: "0 2 * * *", Enabled: true}
	now := time.Date(2026, 7, 3, 17, 30, 0, 0, time.UTC)
	next := info.NextRunAt(now)
	want := time.Date(2026, 7, 3, 18, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next = %s want %s", next, want)
	}
}

func TestInfoNextRunDisabledOrInvalidSpec(t *testing.T) {
	now := time.Date(2026, 7, 4, 1, 30, 0, 0, time.UTC)
	if next := (Info{Spec: "0 2 * * *"}).NextRunAt(now); !next.IsZero() {
		t.Fatalf("disabled next = %s", next)
	}
	if next := (Info{Spec: "bad", Enabled: true}).NextRunAt(now); !next.IsZero() {
		t.Fatalf("invalid next = %s", next)
	}
}

func TestNextRunDescriptorUsesTimezone(t *testing.T) {
	previous := core.Location()
	defer core.SetLocation(previous)
	core.SetLocation(time.UTC)
	now := time.Date(2026, 7, 4, 1, 30, 0, 0, time.UTC)
	for _, spec := range []string{"@daily", "@midnight", "CRON_TZ=Asia/Shanghai @daily", "TZ=Asia/Shanghai @daily"} {
		t.Run(spec, func(t *testing.T) {
			info := Info{Spec: spec, Timezone: "Asia/Shanghai", Enabled: true}
			want := time.Date(2026, 7, 4, 16, 0, 0, 0, time.UTC)
			if next := info.NextRunAt(now); !next.Equal(want) {
				t.Fatalf("next = %s want %s", next, want)
			}
		})
	}
}

func TestFreezeRejectsIncompleteTimezoneSpec(t *testing.T) {
	tasks := task.New()
	tasks.Register[schedulePayload]("sync", func(context.Context, *task.TaskOf[schedulePayload]) error { return nil })
	for _, spec := range []string{"CRON_TZ=UTC", "TZ=Asia/Shanghai"} {
		t.Run(spec, func(t *testing.T) {
			registry := New()
			registry.Register("bad", spec, "sync", schedulePayload{})
			if err := registry.Freeze(tasks); err == nil {
				t.Fatal("expected invalid spec error")
			}
		})
	}
}

func TestInfoPopulatesNextRun(t *testing.T) {
	core.SetClock(core.FixedClock(time.Date(2026, 7, 4, 1, 30, 0, 0, time.UTC)))
	defer core.SetClock(nil)
	registry := New()
	registry.Register("nightly", "0 2 * * *", "sync", schedulePayload{}, Timezone("UTC"))
	registry.Register("paused", "1m", "sync", schedulePayload{}, Enabled(false))
	items := registry.Info()
	want := time.Date(2026, 7, 4, 2, 0, 0, 0, time.UTC)
	if len(items) != 2 || !items[0].NextRun.Equal(want) || !items[1].NextRun.IsZero() {
		t.Fatalf("items = %#v", items)
	}
}
