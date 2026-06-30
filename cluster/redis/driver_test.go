package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa/cluster"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisDriverSharesInstances(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	driver := Driver(client, Prefix("test:cluster"))
	ctx := context.Background()
	if err := driver.Register(ctx, cluster.Instance{ID: "a", Service: "admin", Status: cluster.StatusRunning, TTL: time.Minute}); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := driver.Register(ctx, cluster.Instance{ID: "b", Service: "api", Status: cluster.StatusRunning, TTL: time.Minute}); err != nil {
		t.Fatalf("register b: %v", err)
	}
	items, err := driver.Instances(ctx, "admin")
	if err != nil {
		t.Fatalf("instances: %v", err)
	}
	if len(items) != 1 || items[0].ID != "a" {
		t.Fatalf("items = %#v", items)
	}
	if err := driver.Unregister(ctx, "a"); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	items, err = driver.Instances(ctx, "admin")
	if err != nil {
		t.Fatalf("instances after unregister: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items after unregister = %#v", items)
	}
}
