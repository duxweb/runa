package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa"
	"github.com/duxweb/runa/cache"
	runaprovider "github.com/duxweb/runa/provider"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisStorePrefixAndPurge(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()
	ctx := context.Background()
	store := Driver(client, cache.Prefix("runa:test:"), cache.TTL(time.Minute))
	if err := store.Set(ctx, "a", []byte("1"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := client.Set(ctx, "other:a", "2", 0).Err(); err != nil {
		t.Fatalf("set other: %v", err)
	}
	value, ok, err := store.Get(ctx, "a")
	if err != nil || !ok || string(value) != "1" {
		t.Fatalf("get=%q ok=%v err=%v", value, ok, err)
	}
	if has, err := store.Has(ctx, "a"); err != nil || !has {
		t.Fatalf("has=%v err=%v", has, err)
	}
	values, missing, err := store.GetMany(ctx, []string{"a", "b"})
	if err != nil || string(values["a"]) != "1" || len(missing) != 1 || missing[0] != "b" {
		t.Fatalf("many=%#v missing=%#v err=%v", values, missing, err)
	}
	if err := store.Purge(ctx); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, ok, _ := store.Get(ctx, "a"); ok {
		t.Fatal("prefixed key should be purged")
	}
	if got, err := client.Get(ctx, "other:a").Result(); err != nil || got != "2" {
		t.Fatalf("other key got=%q err=%v", got, err)
	}
}

func TestRedisStoreTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()
	ctx := context.Background()
	store := Driver(client, cache.Prefix("runa:ttl:"))
	if err := store.Set(ctx, "a", []byte("1"), time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	server.FastForward(2 * time.Second)
	if _, ok, err := store.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expired ok=%v err=%v", ok, err)
	}
}

func TestProviderUsesInjectedClientWithoutClosingIt(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	provider := Provider(Client(client), Prefix("provider:cache:"), Name("redis"))
	app := newCacheTestApp(t, provider)
	ctx := context.Background()
	cacheStore := cache.Default().MustOf[string](cache.DefaultName)
	if err := cacheStore.Set(ctx, "a", "1", time.Minute); err != nil {
		t.Fatalf("set: %v", err)
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
	app := newCacheTestApp(t, Provider(Addr(server.Addr()), Prefix("provider:cache:")))
	ctx := context.Background()
	cacheStore := cache.Default().MustOf[string](cache.DefaultName)
	if err := cacheStore.Set(ctx, "a", "1", time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func newCacheTestApp(t *testing.T, provider runaprovider.Provider) *runa.App {
	t.Helper()
	app := runa.New()
	app.Install(
		cache.Provider(
			cache.RegisterPool(cache.DefaultName, cache.Use("redis")),
		),
		provider,
	)
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	return app
}
