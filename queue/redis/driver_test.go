package redis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa"
	runaprovider "github.com/duxweb/runa/provider"
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

func TestRedisDriverConcurrentPushIDsAreUnique(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("unique-ids")).(queue.Driver)
	ctx := context.Background()
	const total = 10000
	const workers = 40
	ids := make(chan string, total)
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for index := range total / workers {
				id, err := driver.Push(ctx, "jobs", &queue.JobMessage{Name: "sync", Payload: []byte(fmt.Sprintf(`{"worker":%d,"index":%d}`, worker, index))})
				if err != nil {
					t.Errorf("push: %v", err)
					return
				}
				ids <- id
			}
		}(worker)
	}
	wait.Wait()
	close(ids)
	seen := make(map[string]struct{}, total)
	for id := range ids {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != total {
		t.Fatalf("ids = %d want %d", len(seen), total)
	}
	pending, err := driver.Count(ctx, "jobs", queue.StatePending)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if pending != total {
		t.Fatalf("pending = %d want %d", pending, total)
	}
}

func TestRedisDriverUniqueStrategiesAndTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("unique")).(queue.Driver)
	ctx := context.Background()

	id, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "done-1", Queue: "jobs", Name: "sync", Unique: "done", UniqueStrategy: string(queue.UniqueStrategyUntilDone)})
	if err != nil {
		t.Fatalf("push until done: %v", err)
	}
	if _, err := driver.Reserve(ctx, "jobs", 1, time.Second); err != nil {
		t.Fatalf("reserve until done: %v", err)
	}
	if err := driver.Fail(ctx, "jobs", id, "failed"); err != nil {
		t.Fatalf("fail until done: %v", err)
	}
	again, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "done-2", Queue: "jobs", Name: "sync", Unique: "done", UniqueStrategy: string(queue.UniqueStrategyUntilDone)})
	if err != nil {
		t.Fatalf("push after fail: %v", err)
	}
	if again != "done-2" {
		t.Fatalf("until done unique was not released on fail: %q", again)
	}

	startID, err := driver.Push(ctx, "start-jobs", &queue.JobMessage{ID: "start-1", Queue: "start-jobs", Name: "sync", Unique: "start", UniqueStrategy: string(queue.UniqueStrategyUntilStart)})
	if err != nil {
		t.Fatalf("push until start: %v", err)
	}
	items, err := driver.Reserve(ctx, "start-jobs", 1, time.Second)
	if err != nil || len(items) != 1 {
		t.Fatalf("reserve until start = %#v err=%v", items, err)
	}
	startAgain, err := driver.Push(ctx, "start-jobs", &queue.JobMessage{ID: "start-2", Queue: "start-jobs", Name: "sync", Unique: "start", UniqueStrategy: string(queue.UniqueStrategyUntilStart)})
	if err != nil {
		t.Fatalf("push after start: %v", err)
	}
	if startAgain == startID {
		t.Fatalf("until start unique was not released on reserve: %q", startAgain)
	}

	ttlID, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "ttl-1", Queue: "jobs", Name: "sync", Unique: "ttl", UniqueStrategy: string(queue.UniqueStrategyUntilDone), UniqueTTL: 20 * time.Millisecond})
	if err != nil {
		t.Fatalf("push ttl: %v", err)
	}
	ttlAgain, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "ttl-2", Queue: "jobs", Name: "sync", Unique: "ttl", UniqueStrategy: string(queue.UniqueStrategyUntilDone), UniqueTTL: 20 * time.Millisecond})
	if err != nil || ttlAgain != ttlID {
		t.Fatalf("ttl before expiry = %q err=%v", ttlAgain, err)
	}
	server.FastForward(30 * time.Millisecond)
	ttlAfter, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "ttl-3", Queue: "jobs", Name: "sync", Unique: "ttl", UniqueStrategy: string(queue.UniqueStrategyUntilDone), UniqueTTL: 20 * time.Millisecond})
	if err != nil {
		t.Fatalf("push ttl after expiry: %v", err)
	}
	if ttlAfter != "ttl-3" {
		t.Fatalf("ttl unique did not expire: %q", ttlAfter)
	}
}

func TestRedisDriverPurgeFailedJobs(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("purge")).(queue.Driver)
	ctx := context.Background()
	id, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "job-1", Queue: "jobs", Name: "sync", Unique: "job", UniqueStrategy: string(queue.UniqueStrategyUntilDone)})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if _, err := driver.Reserve(ctx, "jobs", 1, time.Second); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := driver.Fail(ctx, "jobs", id, "failed"); err != nil {
		t.Fatalf("fail: %v", err)
	}
	count, err := driver.Purge(ctx, "jobs", queue.StateFailed, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if count != 1 {
		t.Fatalf("purged = %d", count)
	}
	failed, _ := driver.Count(ctx, "jobs", queue.StateFailed)
	if failed != 0 {
		t.Fatalf("failed after purge = %d", failed)
	}
}

func TestRedisDriverUniqueTTLReleasesOnFail(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("ttl-fail")).(queue.Driver)
	ctx := context.Background()
	id, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "job-1", Queue: "jobs", Name: "sync", Unique: "job", UniqueStrategy: string(queue.UniqueStrategyUntilDone), UniqueTTL: 30 * time.Millisecond})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if _, err := driver.Reserve(ctx, "jobs", 1, time.Second); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := driver.Fail(ctx, "jobs", id, "failed"); err != nil {
		t.Fatalf("fail: %v", err)
	}
	again, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "job-2", Queue: "jobs", Name: "sync", Unique: "job", UniqueStrategy: string(queue.UniqueStrategyUntilDone), UniqueTTL: 30 * time.Millisecond})
	if err != nil {
		t.Fatalf("push after fail: %v", err)
	}
	if again != "job-2" {
		t.Fatalf("ttl unique was not released after fail: got %q", again)
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

func TestRedisReserveBatchAndConcurrentClaim(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	firstDriver := Driver(client, Prefix("batch")).(queue.Driver)
	secondDriver := Driver(client, Prefix("batch")).(queue.Driver)
	ctx := context.Background()
	for index := range 500 {
		if _, err := firstDriver.Push(ctx, "jobs", &queue.JobMessage{ID: fmt.Sprintf("job-%d", index), Queue: "jobs", Name: "mail", RunAt: time.Now()}); err != nil {
			t.Fatalf("push %d: %v", index, err)
		}
	}
	first, err := firstDriver.Reserve(ctx, "jobs", 50, time.Second)
	if err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	if len(first) != 50 {
		t.Fatalf("first reserve len = %d want 50", len(first))
	}
	seen := make(map[string]struct{}, len(first))
	for _, item := range first {
		if _, ok := seen[item.ID]; ok {
			t.Fatalf("duplicate first id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
	var wait sync.WaitGroup
	results := make(chan []*queue.JobMessage, 2)
	for _, item := range []queue.Driver{firstDriver, secondDriver} {
		wait.Add(1)
		go func(driver queue.Driver) {
			defer wait.Done()
			items, err := driver.Reserve(ctx, "jobs", 50, time.Second)
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			results <- items
		}(item)
	}
	wait.Wait()
	close(results)
	for items := range results {
		for _, item := range items {
			if _, ok := seen[item.ID]; ok {
				t.Fatalf("job consumed twice: %q", item.ID)
			}
			seen[item.ID] = struct{}{}
		}
	}
}

func TestRedisReserveRequeuesExpiredAndIncrementsAttempt(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("expired")).(queue.Driver)
	ctx := context.Background()
	id, err := driver.Push(ctx, "jobs", &queue.JobMessage{Queue: "jobs", Name: "mail", RunAt: time.Now()})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	first, err := driver.Reserve(ctx, "jobs", 1, 10*time.Millisecond)
	if err != nil || len(first) != 1 {
		t.Fatalf("first reserve = %#v err=%v", first, err)
	}
	time.Sleep(15 * time.Millisecond)
	second, err := driver.Reserve(ctx, "jobs", 1, time.Second)
	if err != nil || len(second) != 1 {
		t.Fatalf("second reserve = %#v err=%v", second, err)
	}
	if second[0].ID != id || second[0].Attempt != 2 {
		t.Fatalf("second item = %#v", second[0])
	}
}

func TestRedisReserveRemovesOrphanIDs(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	item := Driver(client, Prefix("orphan")).(*driver)
	ctx := context.Background()
	if err := client.ZAdd(ctx, item.stateKey("jobs", queue.StateReserved), goredis.Z{Score: score(time.Now().Add(-time.Second)), Member: "missing"}).Err(); err != nil {
		t.Fatalf("zadd orphan: %v", err)
	}
	items, err := item.Reserve(ctx, "jobs", 1, time.Second)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %#v", items)
	}
	reserved, err := client.ZCard(ctx, item.stateKey("jobs", queue.StateReserved)).Result()
	if err != nil {
		t.Fatalf("zcard reserved: %v", err)
	}
	pending, err := client.ZCard(ctx, item.stateKey("jobs", queue.StatePending)).Result()
	if err != nil {
		t.Fatalf("zcard pending: %v", err)
	}
	if reserved != 0 || pending != 0 {
		t.Fatalf("orphan remains reserved=%d pending=%d", reserved, pending)
	}
}

func TestRedisReserveRemovesCorruptBodiesWithoutPoisoningBatch(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	item := Driver(client, Prefix("corrupt")).(*driver)
	ctx := context.Background()
	if err := client.HSet(ctx, item.jobKey("bad"), map[string]any{
		jobFieldBody:           "{",
		jobFieldQueue:          "jobs",
		jobFieldName:           "sync",
		jobFieldUnique:         "",
		jobFieldUniqueStrategy: "",
		jobFieldAttempt:        0,
		jobFieldRunAt:          scoreString(time.Now()),
		jobFieldUpdatedAt:      scoreString(time.Now()),
	}).Err(); err != nil {
		t.Fatalf("hset corrupt body: %v", err)
	}
	if err := client.ZAdd(ctx, item.stateKey("jobs", queue.StatePending), goredis.Z{Score: score(time.Now()), Member: "bad"}).Err(); err != nil {
		t.Fatalf("zadd corrupt id: %v", err)
	}
	if _, err := item.Push(ctx, "jobs", &queue.JobMessage{ID: "good", Queue: "jobs", Name: "sync", RunAt: time.Now()}); err != nil {
		t.Fatalf("push good: %v", err)
	}
	items, err := item.Reserve(ctx, "jobs", 2, time.Second)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if len(items) != 1 || items[0].ID != "good" {
		t.Fatalf("items = %#v", items)
	}
	if exists, err := client.Exists(ctx, item.jobKey("bad")).Result(); err != nil || exists != 0 {
		t.Fatalf("corrupt body exists=%d err=%v", exists, err)
	}
	pending, err := client.ZCard(ctx, item.stateKey("jobs", queue.StatePending)).Result()
	if err != nil {
		t.Fatalf("zcard pending: %v", err)
	}
	reserved, err := client.ZCard(ctx, item.stateKey("jobs", queue.StateReserved)).Result()
	if err != nil {
		t.Fatalf("zcard reserved: %v", err)
	}
	if pending != 0 || reserved != 1 {
		t.Fatalf("unexpected states pending=%d reserved=%d", pending, reserved)
	}
}

func TestRedisPurgeFailedJobsInBatches(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	item := Driver(client, Prefix("purge-batch")).(*driver)
	ctx := context.Background()
	now := time.Now().Add(-time.Hour)
	for index := range 5000 {
		id := fmt.Sprintf("failed-%d", index)
		fields, err := jobHashFields(&queue.JobMessage{ID: id, Queue: "jobs", Name: "sync", UpdatedAt: now})
		if err != nil {
			t.Fatalf("hash fields: %v", err)
		}
		if err := client.HSet(ctx, item.jobKey(id), fields).Err(); err != nil {
			t.Fatalf("hset: %v", err)
		}
		if err := client.ZAdd(ctx, item.stateKey("jobs", queue.StateFailed), goredis.Z{Score: score(now), Member: id}).Err(); err != nil {
			t.Fatalf("zadd: %v", err)
		}
	}
	count, err := item.Purge(ctx, "jobs", queue.StateFailed, time.Now())
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if count != 5000 {
		t.Fatalf("purged = %d want 5000", count)
	}
	failed, err := item.Count(ctx, "jobs", queue.StateFailed)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if failed != 0 {
		t.Fatalf("failed after purge = %d", failed)
	}
}

func TestRedisJobBodyIsHashAndStableAcrossStateTransitions(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("hash")).(*driver)
	ctx := context.Background()
	id, err := driver.Push(ctx, "jobs", &queue.JobMessage{ID: "job-1", Queue: "jobs", Name: "sync", Unique: "stable", UniqueStrategy: string(queue.UniqueStrategyUntilDone), RunAt: time.Now()})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	key := driver.jobKey(id)
	keyType, err := client.Type(ctx, key).Result()
	if err != nil {
		t.Fatalf("type: %v", err)
	}
	if keyType != "hash" {
		t.Fatalf("job key type = %q want hash", keyType)
	}
	body, err := client.HGet(ctx, key, jobFieldBody).Bytes()
	if err != nil {
		t.Fatalf("hget body: %v", err)
	}
	if _, err := driver.Reserve(ctx, "jobs", 1, time.Second); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	assertRedisBodyUnchanged(t, client, key, body)
	if err := driver.Release(ctx, "jobs", id, time.Millisecond, "retry"); err != nil {
		t.Fatalf("release: %v", err)
	}
	assertRedisBodyUnchanged(t, client, key, body)
	time.Sleep(2 * time.Millisecond)
	if _, err := driver.Reserve(ctx, "jobs", 1, time.Second); err != nil {
		t.Fatalf("reserve retry: %v", err)
	}
	assertRedisBodyUnchanged(t, client, key, body)
	if err := driver.Fail(ctx, "jobs", id, "failed"); err != nil {
		t.Fatalf("fail: %v", err)
	}
	assertRedisBodyUnchanged(t, client, key, body)
}

func TestRedisRenewDoesNotReviveAckedJob(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("renew")).(queue.Driver)
	ctx := context.Background()
	id, err := driver.Push(ctx, "jobs", &queue.JobMessage{Queue: "jobs", Name: "mail", RunAt: time.Now()})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if _, err := driver.Reserve(ctx, "jobs", 1, time.Second); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := driver.Ack(ctx, "jobs", id); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if err := driver.Renew(ctx, "jobs", id, time.Second); err != nil {
		t.Fatalf("renew after ack: %v", err)
	}
	reserved, err := driver.Count(ctx, "jobs", queue.StateReserved)
	if err != nil {
		t.Fatalf("count reserved: %v", err)
	}
	if reserved != 0 {
		t.Fatalf("reserved after ack+renew = %d", reserved)
	}
}

func TestRedisSweepLock(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("sweep")).(*driver)
	ctx := context.Background()
	locked, err := driver.LockSweep(ctx, "jobs", time.Second)
	if err != nil || !locked {
		t.Fatalf("first lock = %v err=%v", locked, err)
	}
	locked, err = driver.LockSweep(ctx, "jobs", time.Second)
	if err != nil {
		t.Fatalf("second lock: %v", err)
	}
	if locked {
		t.Fatal("second lock should be skipped")
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

func TestProviderUsesInjectedClientWithoutClosingIt(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	app := runa.New()
	app.Install(
		queue.Provider(
			queue.RegisterQueue("jobs", queue.Use("redis"), queue.Workers("jobs")),
			queue.RegisterWorker("jobs"),
		),
		Provider(Client(client), Prefix("provider:injected")),
	)
	app.Module(providerTestModule{})
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	id, err := queue.Default().Push(ctx, "jobs", "provider-test", map[string]int{"id": 1})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if id == "" {
		t.Fatal("id is empty")
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("injected client should remain open: %v", err)
	}
	_ = client.Close()
}

func TestProviderUsesExplicitRedisOptions(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	app := runa.New()
	app.Install(
		queue.Provider(
			queue.RegisterQueue("jobs", queue.Use("redis"), queue.Workers("jobs")),
			queue.RegisterWorker("jobs"),
		),
		Provider(Addr(server.Addr()), Prefix("provider:options")),
	)
	app.Module(providerTestModule{})
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	id, err := queue.Default().Push(ctx, "jobs", "provider-test", map[string]int{"id": 1})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if id == "" {
		t.Fatal("id is empty")
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestProviderUsesFeatureRedisConfig(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	root := t.TempDir()
	writeConfig(t, root, "queue.toml", "[redis]\naddr = '"+server.Addr()+"'\nprefix = 'provider:feature'\n")
	app := runa.New(runa.BasePath(root), runa.ConfigPath("config"))
	app.Install(
		queue.Provider(
			queue.RegisterQueue("jobs", queue.Use("redis"), queue.Workers("jobs")),
			queue.RegisterWorker("jobs"),
		),
		Provider(),
	)
	app.Module(providerTestModule{})
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	id, err := queue.Default().Push(ctx, "jobs", "provider-test", map[string]int{"id": 1})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if id == "" {
		t.Fatal("id is empty")
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestProviderUsesSharedRedisConfig(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	root := t.TempDir()
	writeConfig(t, root, "redis.toml", "addr = '"+server.Addr()+"'\n")
	app := runa.New(runa.BasePath(root), runa.ConfigPath("config"))
	app.Install(
		queue.Provider(
			queue.RegisterQueue("jobs", queue.Use("redis"), queue.Workers("jobs")),
			queue.RegisterWorker("jobs"),
		),
		Provider(Prefix("provider:shared")),
	)
	app.Module(providerTestModule{})
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	id, err := queue.Default().Push(ctx, "jobs", "provider-test", map[string]int{"id": 1})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if id == "" {
		t.Fatal("id is empty")
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestProviderUsesNamedSharedRedisConfig(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	root := t.TempDir()
	writeConfig(t, root, "redis.toml", "[queue]\naddr = '"+server.Addr()+"'\n")
	app := runa.New(runa.BasePath(root), runa.ConfigPath("config"))
	app.Install(
		queue.Provider(
			queue.RegisterQueue("jobs", queue.Use("redis"), queue.Workers("jobs")),
			queue.RegisterWorker("jobs"),
		),
		Provider(Use("queue"), Prefix("provider:named")),
	)
	app.Module(providerTestModule{})
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	id, err := queue.Default().Push(ctx, "jobs", "provider-test", map[string]int{"id": 1})
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if id == "" {
		t.Fatal("id is empty")
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestProviderFeatureConfigCanOverrideSharedRedisZeroValues(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	root := t.TempDir()
	writeConfig(t, root, "redis.toml", "addr = '"+server.Addr()+"'\ndb = 1\npool_size = 8\n")
	writeConfig(t, root, "queue.toml", "[redis]\ndb = 0\npool_size = 0\n")
	app := runa.New(runa.BasePath(root), runa.ConfigPath("config"))
	app.Install(
		queue.Provider(
			queue.RegisterQueue("jobs", queue.Use("redis"), queue.Workers("jobs")),
			queue.RegisterWorker("jobs"),
		),
		Provider(Prefix("provider:zero")),
	)
	app.Module(providerTestModule{})
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := queue.Default().Push(ctx, "jobs", "provider-test", map[string]int{"id": 1}); err != nil {
		t.Fatalf("push: %v", err)
	}
	db0 := goredis.NewClient(&goredis.Options{Addr: server.Addr(), DB: 0})
	defer db0.Close()
	db1 := goredis.NewClient(&goredis.Options{Addr: server.Addr(), DB: 1})
	defer db1.Close()
	db0Keys, err := db0.Keys(ctx, "provider:zero:*").Result()
	if err != nil {
		t.Fatalf("db0 keys: %v", err)
	}
	db1Keys, err := db1.Keys(ctx, "provider:zero:*").Result()
	if err != nil {
		t.Fatalf("db1 keys: %v", err)
	}
	if len(db0Keys) == 0 {
		t.Fatalf("expected queue keys in db0")
	}
	if len(db1Keys) != 0 {
		t.Fatalf("expected no queue keys in db1, got %#v", db1Keys)
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestProviderIgnoresSharedRedisPrefix(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	root := t.TempDir()
	writeConfig(t, root, "redis.toml", "addr = '"+server.Addr()+"'\nprefix = 'shared:bad'\n")
	writeConfig(t, root, "queue.toml", "[redis]\nprefix = 'feature:queue'\n")
	app := runa.New(runa.BasePath(root), runa.ConfigPath("config"))
	app.Install(
		queue.Provider(
			queue.RegisterQueue("jobs", queue.Use("redis"), queue.Workers("jobs")),
			queue.RegisterWorker("jobs"),
		),
		Provider(),
	)
	app.Module(providerTestModule{})
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := queue.Default().Push(ctx, "jobs", "provider-test", map[string]int{"id": 1}); err != nil {
		t.Fatalf("push: %v", err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()
	featureKeys, err := client.Keys(ctx, "feature:queue:*").Result()
	if err != nil {
		t.Fatalf("feature keys: %v", err)
	}
	sharedKeys, err := client.Keys(ctx, "shared:bad:*").Result()
	if err != nil {
		t.Fatalf("shared keys: %v", err)
	}
	if len(featureKeys) == 0 {
		t.Fatal("expected feature queue prefix to be used")
	}
	if len(sharedKeys) != 0 {
		t.Fatalf("shared redis prefix should be ignored, got %#v", sharedKeys)
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func writeConfig(t *testing.T, root string, name string, body string) {
	t.Helper()
	path := filepath.Join(root, "config")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func assertRedisBodyUnchanged(t *testing.T, client *goredis.Client, key string, want []byte) {
	t.Helper()
	got, err := client.HGet(context.Background(), key, jobFieldBody).Bytes()
	if err != nil {
		t.Fatalf("hget body: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("job body was rewritten\ngot:  %s\nwant: %s", got, want)
	}
}

type providerTestModule struct{}

func (providerTestModule) Name() string { return "queue-redis-provider-test" }

func (providerTestModule) Init(context.Context, runaprovider.Context) error { return nil }
func (providerTestModule) Boot(context.Context, runaprovider.Context) error { return nil }
func (providerTestModule) Shutdown(context.Context, runaprovider.Context) error {
	return nil
}

func (providerTestModule) Register(ctx context.Context, app runaprovider.Context) error {
	queue.Default().Job[map[string]int]("provider-test", func(context.Context, *queue.Job[map[string]int]) error { return nil })
	return nil
}
