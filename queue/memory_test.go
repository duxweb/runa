package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryPayload struct {
	ID int `json:"id"`
}

func TestMemoryDriverLifecycle(t *testing.T) {
	ctx := context.Background()
	driver := MemoryDriver()
	job := &JobMessage{ID: "job-1", Queue: "default", Name: "mail.send", Payload: []byte(`{"id":1}`), RunAt: time.Now(), MaxAttempt: 1}
	id, err := driver.Push(ctx, "default", job)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if id != "job-1" {
		t.Fatalf("id = %q", id)
	}
	pending, _ := driver.Count(ctx, "default", StatePending)
	if pending != 1 {
		t.Fatalf("pending = %d", pending)
	}
	items, err := driver.Reserve(ctx, "default", 1, time.Second)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if len(items) != 1 || items[0].Attempt != 1 || items[0].ReservedUntil.IsZero() {
		t.Fatalf("reserved = %#v", items)
	}
	reserved, _ := driver.Count(ctx, "default", StateReserved)
	if reserved != 1 {
		t.Fatalf("reserved count = %d", reserved)
	}
	if err := driver.Ack(ctx, "default", id); err != nil {
		t.Fatalf("ack: %v", err)
	}
	pending, _ = driver.Count(ctx, "default", StatePending)
	if pending != 0 {
		t.Fatalf("pending after ack = %d", pending)
	}
}

func TestMemoryDriverDelayRetryUniqueRenewAndFail(t *testing.T) {
	ctx := context.Background()
	driver := MemoryDriver()
	runAt := time.Now().Add(40 * time.Millisecond)
	job := &JobMessage{ID: "delay-1", Queue: "jobs", Name: "image.resize", RunAt: runAt, Unique: "image:1", MaxAttempt: 1}
	id, err := driver.Push(ctx, "jobs", job)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	again, err := driver.Push(ctx, "jobs", &JobMessage{ID: "delay-2", Queue: "jobs", Name: "image.resize", Unique: "image:1"})
	if err != nil {
		t.Fatalf("unique push: %v", err)
	}
	if again != id {
		t.Fatalf("unique id = %q want %q", again, id)
	}
	delayed, _ := driver.Count(ctx, "jobs", StateDelayed)
	if delayed != 1 {
		t.Fatalf("delayed = %d", delayed)
	}
	if items, _ := driver.Reserve(ctx, "jobs", 1, 15*time.Millisecond); len(items) != 0 {
		t.Fatalf("reserved delayed = %#v", items)
	}
	time.Sleep(50 * time.Millisecond)
	items, err := driver.Reserve(ctx, "jobs", 1, 15*time.Millisecond)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v", items)
	}
	if err := driver.Renew(ctx, "jobs", id, time.Second); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if err := driver.Release(ctx, "jobs", id, 20*time.Millisecond, "retry"); err != nil {
		t.Fatalf("release: %v", err)
	}
	items, _ = driver.Reserve(ctx, "jobs", 1, time.Second)
	if len(items) != 0 {
		t.Fatalf("reserved before retry delay = %#v", items)
	}
	time.Sleep(25 * time.Millisecond)
	items, _ = driver.Reserve(ctx, "jobs", 1, time.Second)
	if len(items) != 1 || items[0].LastError != "retry" || items[0].Attempt != 2 {
		t.Fatalf("retry item = %#v", items)
	}
	if err := driver.Fail(ctx, "jobs", id, "failed"); err != nil {
		t.Fatalf("fail: %v", err)
	}
	failed, _ := driver.Count(ctx, "jobs", StateFailed)
	if failed != 1 {
		t.Fatalf("failed = %d", failed)
	}
	list, err := driver.List(ctx, "jobs", JobQuery{State: StateFailed, Page: 1, Limit: 10})
	if err != nil || len(list) != 1 || list[0].LastError != "failed" {
		t.Fatalf("list = %#v err=%v", list, err)
	}
}

func TestRegistryWorkerRunsJobsAndRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry := New()
	registry.Queue("jobs", Retry(1), RetryDelay(10*time.Millisecond), Workers("jobs"))
	registry.Worker("jobs", PollInterval(5*time.Millisecond), Lease(50*time.Millisecond))
	var attempts int
	done := make(chan struct{})
	registry.Job[memoryPayload]("unstable", func(ctx context.Context, job *Job[memoryPayload]) error {
		attempts++
		if attempts == 1 {
			return errors.New("try again")
		}
		close(done)
		return nil
	})
	if err := registry.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := registry.Push(ctx, "jobs", "unstable", memoryPayload{ID: 7}); err != nil {
		t.Fatalf("push: %v", err)
	}
	unit := NewUnit(registry, "jobs")
	if err := unit.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer unit.Stop(context.Background())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job not executed")
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	info := workerInfoByName(registry.WorkerInfo(ctx), "jobs")
	if info.Name == "" || info.Processed < 2 || info.Retried != 1 || info.Succeeded != 1 {
		t.Fatalf("worker info = %#v", info)
	}
}

func TestRegistryWorkerFailsAfterRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry := New()
	registry.Queue("jobs", Retry(1), RetryDelay(time.Millisecond), Workers("jobs"))
	registry.Worker("jobs", PollInterval(time.Millisecond), Lease(50*time.Millisecond))
	var attempts int
	done := make(chan struct{})
	registry.Job[memoryPayload]("always-fail", func(ctx context.Context, job *Job[memoryPayload]) error {
		attempts++
		if attempts == 2 {
			close(done)
		}
		return errors.New("failed")
	})
	if err := registry.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := registry.Push(ctx, "jobs", "always-fail", memoryPayload{ID: 7}); err != nil {
		t.Fatalf("push: %v", err)
	}
	unit := NewUnit(registry, "jobs")
	if err := unit.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer unit.Stop(context.Background())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job not retried")
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		info := registry.QueueInfo(ctx)
		if len(info) > 0 && info[0].Failed == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	queueInfo := queueInfoByName(registry.QueueInfo(ctx), "jobs")
	if queueInfo.Failed != 1 {
		t.Fatalf("queue info = %#v", queueInfo)
	}
	workerInfo := workerInfoByName(registry.WorkerInfo(ctx), "jobs")
	if workerInfo.Processed < 2 || workerInfo.Retried != 1 || workerInfo.Failed != 1 {
		t.Fatalf("worker info = %#v", workerInfo)
	}
}

func TestMemoryWorkerState(t *testing.T) {
	ctx := context.Background()
	driver := MemoryDriver().(WorkerState)
	instance := WorkerInstance{ID: "worker-1", Worker: "default", Concurrency: 2}
	if err := driver.Register(ctx, instance); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := driver.Heartbeat(ctx, "worker-1"); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	items, err := driver.Instances(ctx, "default")
	if err != nil || len(items) != 1 || items[0].ID != "worker-1" {
		t.Fatalf("instances = %#v err=%v", items, err)
	}
	if err := driver.Unregister(ctx, "worker-1"); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	items, _ = driver.Instances(ctx, "default")
	if len(items) != 0 {
		t.Fatalf("instances after unregister = %#v", items)
	}
}

func TestWorkerConcurrency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry := New()
	registry.Queue("jobs", Workers("jobs"))
	registry.Worker("jobs", Concurrency(2), PollInterval(time.Millisecond))
	var mu sync.Mutex
	count := 0
	done := make(chan struct{})
	registry.Job[memoryPayload]("count", func(ctx context.Context, job *Job[memoryPayload]) error {
		mu.Lock()
		count++
		if count == 2 {
			close(done)
		}
		mu.Unlock()
		return nil
	})
	if err := registry.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := registry.Push(ctx, "jobs", "count", memoryPayload{ID: 1}); err != nil {
		t.Fatalf("push 1: %v", err)
	}
	if _, err := registry.Push(ctx, "jobs", "count", memoryPayload{ID: 2}); err != nil {
		t.Fatalf("push 2: %v", err)
	}
	unit := NewUnit(registry, "jobs")
	if err := unit.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer unit.Stop(context.Background())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("jobs not executed")
	}
}

func workerInfoByName(items []WorkerInfo, name string) WorkerInfo {
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	return WorkerInfo{}
}

func queueInfoByName(items []QueueInfo, name string) QueueInfo {
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	return QueueInfo{}
}
