package redis

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/duxweb/runa/queue"
	goredis "github.com/redis/go-redis/v9"
)

const redisBenchAddrEnv = "RUNA_REDIS_BENCH_ADDR"

func BenchmarkPush(b *testing.B) {
	ctx := context.Background()
	client := redisBenchClient(b)
	driver := Driver(client, Prefix(redisBenchPrefix(b))).(queue.Driver)
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		if _, err := driver.Push(ctx, "jobs", &queue.JobMessage{Name: "bench", Payload: []byte(fmt.Sprintf(`{"id":%d}`, index))}); err != nil {
			b.Fatalf("push: %v", err)
		}
	}
}

func BenchmarkReserveAck(b *testing.B) {
	ctx := context.Background()
	client := redisBenchClient(b)
	driver := Driver(client, Prefix(redisBenchPrefix(b))).(queue.Driver)
	for index := range b.N {
		if _, err := driver.Push(ctx, "jobs", &queue.JobMessage{Name: "bench", Payload: []byte(fmt.Sprintf(`{"id":%d}`, index))}); err != nil {
			b.Fatalf("push setup: %v", err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	processed := 0
	for processed < b.N {
		items, err := driver.Reserve(ctx, "jobs", maxReserveLimit, time.Minute)
		if err != nil {
			b.Fatalf("reserve: %v", err)
		}
		if len(items) == 0 {
			b.Fatalf("reserve returned no jobs after %d/%d", processed, b.N)
		}
		for _, item := range items {
			if err := driver.Ack(ctx, item.Queue, item.ID); err != nil {
				b.Fatalf("ack: %v", err)
			}
			processed++
		}
	}
}

func BenchmarkWorkerThroughput(b *testing.B) {
	ctx := context.Background()
	client := redisBenchClient(b)
	registry := queue.New()
	registry.RegisterDriver("redis", Driver(client, Prefix(redisBenchPrefix(b))).(queue.Driver))
	registry.Queue("jobs", queue.Use("redis"), queue.Workers("jobs"), queue.Retention(0))
	registry.Worker("jobs", queue.Concurrency(50), queue.PollInterval(time.Millisecond), queue.Lease(time.Minute))
	registry.Job[map[string]int]("bench", func(context.Context, *queue.Job[map[string]int]) error { return nil })
	if err := registry.Freeze(); err != nil {
		b.Fatalf("freeze: %v", err)
	}
	for index := range b.N {
		if _, err := registry.Push(ctx, "jobs", "bench", map[string]int{"id": index}); err != nil {
			b.Fatalf("push setup: %v", err)
		}
	}
	unit := queue.NewUnit(registry, "jobs")
	b.ReportAllocs()
	b.ResetTimer()
	if err := unit.Start(ctx); err != nil {
		b.Fatalf("start worker: %v", err)
	}
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		for _, info := range registry.WorkerInfo(ctx) {
			if info.Name == "jobs" && int(info.Succeeded) >= b.N {
				b.StopTimer()
				if err := unit.Stop(context.Background()); err != nil {
					b.Fatalf("stop worker: %v", err)
				}
				return
			}
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			b.StopTimer()
			_ = unit.Stop(context.Background())
			b.Fatalf("processed %d/%d before timeout", redisBenchSucceeded(ctx, registry), b.N)
		}
	}
}

func redisBenchClient(b *testing.B) *goredis.Client {
	b.Helper()
	addr := os.Getenv(redisBenchAddrEnv)
	if addr == "" {
		b.Skipf("set %s to run Redis benchmarks", redisBenchAddrEnv)
	}
	client := goredis.NewClient(&goredis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		b.Fatalf("ping redis: %v", err)
	}
	prefix := redisBenchPrefix(b)
	b.Cleanup(func() {
		ctx := context.Background()
		keys, err := client.Keys(ctx, prefix+":*").Result()
		if err == nil && len(keys) > 0 {
			_ = client.Del(ctx, keys...).Err()
		}
		_ = client.Close()
	})
	return client
}

func redisBenchPrefix(b *testing.B) string {
	b.Helper()
	return "bench:" + b.Name()
}

func redisBenchSucceeded(ctx context.Context, registry *queue.Registry) int64 {
	for _, info := range registry.WorkerInfo(ctx) {
		if info.Name == "jobs" {
			return info.Succeeded
		}
	}
	return 0
}
