package cache

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type profile struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestTypedCacheSetGetRememberAndMany(t *testing.T) {
	registry := New()
	pool := registry.MustOf[profile](DefaultName)
	ctx := context.Background()
	if err := pool.Set(ctx, "user:1", profile{Name: "Runa", Age: 1}, time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	value, ok, err := pool.Get(ctx, "user:1")
	if err != nil || !ok || value.Name != "Runa" || value.Age != 1 {
		t.Fatalf("get = %#v %v %v", value, ok, err)
	}
	values, missing, err := pool.GetMany(ctx, []string{"user:1", "user:2"})
	if err != nil {
		t.Fatalf("many: %v", err)
	}
	if values["user:1"].Name != "Runa" || len(missing) != 1 || missing[0] != "user:2" {
		t.Fatalf("many values=%#v missing=%#v", values, missing)
	}
	var calls atomic.Int64
	loaded, err := pool.Remember(ctx, "user:2", time.Minute, func(context.Context) (profile, error) {
		calls.Add(1)
		return profile{Name: "Cache", Age: 2}, nil
	})
	if err != nil || loaded.Name != "Cache" || calls.Load() != 1 {
		t.Fatalf("remember loaded=%#v calls=%d err=%v", loaded, calls.Load(), err)
	}
	cached, err := pool.Remember(ctx, "user:2", time.Minute, func(context.Context) (profile, error) {
		calls.Add(1)
		return profile{Name: "Wrong"}, nil
	})
	if err != nil || cached.Name != "Cache" || calls.Load() != 1 {
		t.Fatalf("remember cached=%#v calls=%d err=%v", cached, calls.Load(), err)
	}
}

func TestRememberSingleFlight(t *testing.T) {
	registry := New()
	pool := registry.MustOf[string](DefaultName)
	ctx := context.Background()
	var calls atomic.Int64
	var wait sync.WaitGroup
	for range 10 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value, err := pool.Remember(ctx, "same", time.Minute, func(context.Context) (string, error) {
				calls.Add(1)
				time.Sleep(20 * time.Millisecond)
				return "ok", nil
			})
			if err != nil || value != "ok" {
				t.Errorf("value=%q err=%v", value, err)
			}
		}()
	}
	wait.Wait()
	if calls.Load() != 1 {
		t.Fatalf("loader calls = %d", calls.Load())
	}
}

func TestRememberLoaderPanicDoesNotPoisonKey(t *testing.T) {
	registry := New()
	pool := registry.MustOf[string](DefaultName)
	ctx := context.Background()

	_, err := pool.Remember(ctx, "panic", time.Minute, func(context.Context) (string, error) {
		panic("boom")
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("panic err = %v", err)
	}

	value, err := pool.Remember(ctx, "panic", time.Minute, func(context.Context) (string, error) {
		return "ok", nil
	})
	if err != nil || value != "ok" {
		t.Fatalf("value=%q err=%v", value, err)
	}
}

func TestRegistryCreatesTypedCache(t *testing.T) {
	registry := New()
	registry.Cache("direct", Prefix("direct:"), TTL(time.Minute))
	pool := registry.MustOf[profile]("direct")
	ctx := context.Background()
	if err := pool.Set(ctx, "user:1", profile{Name: "Direct", Age: 3}, 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	value, ok, err := pool.Get(ctx, "user:1")
	if err != nil || !ok || value.Name != "Direct" || value.Age != 3 {
		t.Fatalf("get = %#v %v %v", value, ok, err)
	}
}

func TestMemoryDriverTTLDeleteAndPurge(t *testing.T) {
	store := MemoryDriver(Capacity(1024*1024), TTL(time.Second))
	ctx := context.Background()
	if err := store.Set(ctx, "a", []byte("1"), time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	if value, ok, err := store.Get(ctx, "a"); err != nil || !ok || string(value) != "1" {
		t.Fatalf("get = %q %v %v", value, ok, err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, ok, err := store.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expired ok=%v err=%v", ok, err)
	}
	if err := store.Set(ctx, "b", []byte("2"), time.Minute); err != nil {
		t.Fatalf("set b: %v", err)
	}
	if err := store.Delete(ctx, "b"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, _ := store.Get(ctx, "b"); ok {
		t.Fatal("b should be deleted")
	}
	if err := store.Set(ctx, "c", []byte("3"), time.Minute); err != nil {
		t.Fatalf("set c: %v", err)
	}
	if err := store.Purge(ctx); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, ok, _ := store.Get(ctx, "c"); ok {
		t.Fatal("c should be purged")
	}
}

func TestLayeredDriverReadsL2AndBackfillsL1(t *testing.T) {
	ctx := context.Background()
	l1 := MemoryDriver(Capacity(1024*1024), TTL(time.Minute))
	l2 := MemoryDriver(Capacity(1024*1024), Prefix("l2:"), TTL(time.Minute))
	layered := LayeredDriver(l1, l2, TTL(time.Minute))
	if err := l2.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("l2 set: %v", err)
	}
	value, ok, err := layered.Get(ctx, "key")
	if err != nil || !ok || string(value) != "value" {
		t.Fatalf("layer get=%q ok=%v err=%v", value, ok, err)
	}
	if value, ok, err := l1.Get(ctx, "key"); err != nil || !ok || string(value) != "value" {
		t.Fatalf("l1 backfill=%q ok=%v err=%v", value, ok, err)
	}
}
