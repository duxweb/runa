package redis

import (
	"context"
	"os"
	"path/filepath"
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
