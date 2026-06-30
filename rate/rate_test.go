package rate

import (
	"context"
	"testing"
	"time"
)

func TestMemoryFixedWindowLimitAndReset(t *testing.T) {
	registry := New()
	registry.Rate("login", FixedWindow(2, time.Minute))
	limiter := registry.MustOf("login")
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		result, err := limiter.Allow(ctx, "ip:1")
		if err != nil || !result.Allowed {
			t.Fatalf("allow %d result=%+v err=%v", i, result, err)
		}
	}
	result, err := limiter.Allow(ctx, "ip:1")
	if err != nil {
		t.Fatalf("allow blocked: %v", err)
	}
	if result.Allowed || result.Remaining != 0 || result.RetryAfter <= 0 {
		t.Fatalf("expected blocked result: %+v", result)
	}
	if err := limiter.Reset(ctx, "ip:1"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	result, err = limiter.Allow(ctx, "ip:1")
	if err != nil || !result.Allowed {
		t.Fatalf("allow after reset result=%+v err=%v", result, err)
	}
}

func TestMemoryTokenBucket(t *testing.T) {
	registry := New()
	registry.Rate("api", TokenBucket(1, 50*time.Millisecond), Burst(1))
	limiter := registry.MustOf("api")
	ctx := context.Background()

	first, _ := limiter.Allow(ctx, "k")
	second, _ := limiter.Allow(ctx, "k")
	if !first.Allowed || second.Allowed {
		t.Fatalf("unexpected results first=%+v second=%+v", first, second)
	}
	time.Sleep(60 * time.Millisecond)
	third, err := limiter.Allow(ctx, "k")
	if err != nil || !third.Allowed {
		t.Fatalf("expected refill result=%+v err=%v", third, err)
	}
}

func TestMemorySlidingWindow(t *testing.T) {
	registry := New()
	registry.Rate("slide", SlidingWindow(1, time.Minute))
	limiter := registry.MustOf("slide")
	ctx := context.Background()
	first, _ := limiter.Allow(ctx, "k")
	second, _ := limiter.Allow(ctx, "k")
	if !first.Allowed || second.Allowed {
		t.Fatalf("unexpected sliding results first=%+v second=%+v", first, second)
	}
}

func TestRegistryCreatesLimiter(t *testing.T) {
	registry := New()
	registry.Rate("direct", FixedWindow(1, time.Minute))
	limiter := registry.MustOf("direct")
	ctx := context.Background()
	first, err := limiter.Allow(ctx, "direct")
	if err != nil || !first.Allowed {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := limiter.Allow(ctx, "direct")
	if err != nil || second.Allowed {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestRegistryInfoAndExternalDriver(t *testing.T) {
	registry := New()
	driver := &fakeDriver{}
	registry.RegisterDriver("fake", driver)
	registry.Rate("custom", Use("fake"), FixedWindow(9, time.Minute), Meta("module", "test"))
	limiter := registry.MustOf("custom")
	result, err := limiter.Allow(context.Background(), "a", "b")
	if err != nil || !result.Allowed {
		t.Fatalf("allow result=%+v err=%v", result, err)
	}
	if driver.key != "custom:a:b" {
		t.Fatalf("unexpected key %q", driver.key)
	}
	infos := registry.Info()
	var found Info
	for _, item := range infos {
		if item.Name == "custom" {
			found = item
		}
	}
	if found.Name != "custom" || found.Driver != "fake" || found.Meta["module"] != "test" {
		t.Fatalf("unexpected info: %+v", found)
	}
}

type fakeDriver struct{ key string }

func (driver *fakeDriver) Name() string { return "fake" }
func (driver *fakeDriver) Allow(_ context.Context, rule Rule, key string) (Result, error) {
	driver.key = key
	return Result{Allowed: true, Limit: rule.Limit, Remaining: rule.Limit - 1, ResetAt: time.Now().Add(rule.Window)}, nil
}
func (driver *fakeDriver) Reset(context.Context, Rule, string) error { return nil }
func (driver *fakeDriver) Close(context.Context) error               { return nil }
