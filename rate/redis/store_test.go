package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa/rate"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisDriverFixedWindowAndReset(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, rate.Prefix("test:rate:"))
	rule := rate.Rule{Name: "login", Algorithm: rate.AlgorithmFixedWindow, Limit: 2, Window: time.Minute, Burst: 2}
	ctx := context.Background()

	first, err := driver.Allow(ctx, rule, "ip:1")
	if err != nil || !first.Allowed {
		t.Fatalf("first result=%+v err=%v", first, err)
	}
	second, _ := driver.Allow(ctx, rule, "ip:1")
	third, _ := driver.Allow(ctx, rule, "ip:1")
	if !second.Allowed || third.Allowed || third.RetryAfter <= 0 {
		t.Fatalf("unexpected results second=%+v third=%+v", second, third)
	}
	if err := driver.Reset(ctx, rule, "ip:1"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	after, _ := driver.Allow(ctx, rule, "ip:1")
	if !after.Allowed {
		t.Fatalf("expected allowed after reset: %+v", after)
	}
}

func TestRedisDriverTokenBucketAndSlidingWindow(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client)
	ctx := context.Background()

	bucket := rate.Rule{Name: "api", Algorithm: rate.AlgorithmTokenBucket, Limit: 1, Window: time.Minute, Burst: 1}
	first, _ := driver.Allow(ctx, bucket, "bucket")
	second, _ := driver.Allow(ctx, bucket, "bucket")
	if !first.Allowed || second.Allowed {
		t.Fatalf("unexpected bucket results first=%+v second=%+v", first, second)
	}

	sliding := rate.Rule{Name: "slide", Algorithm: rate.AlgorithmSlidingWindow, Limit: 1, Window: time.Minute, Burst: 1}
	first, _ = driver.Allow(ctx, sliding, "slide")
	second, _ = driver.Allow(ctx, sliding, "slide")
	if !first.Allowed || second.Allowed {
		t.Fatalf("unexpected sliding results first=%+v second=%+v", first, second)
	}
}
