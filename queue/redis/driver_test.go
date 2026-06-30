package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa/queue"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisDriverLifecycle(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("test")).(queue.Driver)
	ctx := context.Background()
	id, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "job-1", Queue: "jobs", Name: "mail", Payload: []byte(`{"id":1}`), RunAt: time.Now(), Unique: "mail:1"})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	again, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "job-2", Queue: "jobs", Name: "mail", RunAt: time.Now(), Unique: "mail:1"})
	if err != nil || again != id {
		t.Fatalf("unique id = %q err=%v", again, err)
	}
	pending, _ := driver.Count(ctx, "jobs", queue.StatePending)
	if pending != 1 {
		t.Fatalf("pending = %d", pending)
	}
	items, err := driver.Reserve(ctx, "jobs", 1, time.Second)
	if err != nil || len(items) != 1 || items[0].Attempt != 1 {
		t.Fatalf("reserve = %#v err=%v", items, err)
	}
	if err := driver.Release(ctx, "jobs", id, 10*time.Millisecond, "retry"); err != nil {
		t.Fatalf("release: %v", err)
	}
	delayed, _ := driver.Count(ctx, "jobs", queue.StateDelayed)
	if delayed != 1 {
		t.Fatalf("delayed = %d", delayed)
	}
	time.Sleep(15 * time.Millisecond)
	items, err = driver.Reserve(ctx, "jobs", 1, time.Second)
	if err != nil || len(items) != 1 || items[0].Attempt != 2 || items[0].LastError != "retry" {
		t.Fatalf("retry reserve = %#v err=%v", items, err)
	}
	if err := driver.Fail(ctx, "jobs", id, "failed"); err != nil {
		t.Fatalf("fail: %v", err)
	}
	failed, _ := driver.Count(ctx, "jobs", queue.StateFailed)
	if failed != 1 {
		t.Fatalf("failed = %d", failed)
	}
	list, err := driver.List(ctx, "jobs", queue.JobQuery{State: queue.StateFailed, Page: 1, Limit: 10})
	if err != nil || len(list) != 1 || list[0].LastError != "failed" {
		t.Fatalf("list = %#v err=%v", list, err)
	}
	if err := driver.Delete(ctx, "jobs", id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	failed, _ = driver.Count(ctx, "jobs", queue.StateFailed)
	if failed != 0 {
		t.Fatalf("failed after delete = %d", failed)
	}
}

func TestRedisReserveOnlyReturnsClaimedJobs(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("claim")).(queue.Driver)
	ctx := context.Background()
	if _, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "job-1", Queue: "jobs", Name: "mail", RunAt: time.Now()}); err != nil {
		t.Fatalf("push: %v", err)
	}
	first, err := driver.Reserve(ctx, "jobs", 1, time.Second)
	if err != nil || len(first) != 1 {
		t.Fatalf("first = %#v err=%v", first, err)
	}
	second, err := driver.Reserve(ctx, "jobs", 1, time.Second)
	if err != nil {
		t.Fatalf("second reserve: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("second = %#v", second)
	}
}

func TestRedisWorkerState(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("state")).(queue.WorkerState)
	ctx := context.Background()
	instance := queue.WorkerInstance{ID: "worker-1", Worker: "mail", Concurrency: 3}
	if err := driver.Register(ctx, instance); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := driver.Heartbeat(ctx, "worker-1"); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	items, err := driver.Instances(ctx, "mail")
	if err != nil || len(items) != 1 || items[0].ID != "worker-1" {
		t.Fatalf("instances = %#v err=%v", items, err)
	}
	if err := driver.Unregister(ctx, "worker-1"); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	items, _ = driver.Instances(ctx, "mail")
	if len(items) != 0 {
		t.Fatalf("instances after unregister = %#v", items)
	}
}
