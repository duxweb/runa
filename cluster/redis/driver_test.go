package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa"
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

func TestProviderUsesInjectedClientWithoutClosingIt(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	app := runa.New()
	app.Install(
		cluster.Provider(cluster.UseDriver("redis"), cluster.ID("node-1"), cluster.Service("api"), cluster.TTL(time.Minute)),
		Provider(Client(client), Prefix("provider:cluster")),
	)
	ctx := context.Background()
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	items, err := cluster.Default().Instances(ctx, "api")
	if err != nil || len(items) != 1 || items[0].ID != "node-1" {
		t.Fatalf("instances=%#v err=%v", items, err)
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("injected client should remain open: %v", err)
	}
	_ = client.Close()
}

func TestProviderUsesExplicitOptions(t *testing.T) {
	server := miniredis.RunT(t)
	app := runa.New()
	app.Install(
		cluster.Provider(cluster.UseDriver("redis"), cluster.ID("node-1"), cluster.Service("api"), cluster.TTL(time.Minute)),
		Provider(Addr(server.Addr()), Prefix("provider:cluster")),
	)
	ctx := context.Background()
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	items, err := cluster.Default().Instances(ctx, "api")
	if err != nil || len(items) != 1 || items[0].ID != "node-1" {
		t.Fatalf("instances=%#v err=%v", items, err)
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
