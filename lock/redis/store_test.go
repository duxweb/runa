package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa"
	"github.com/duxweb/runa/lock"
	runaprovider "github.com/duxweb/runa/provider"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisStoreTryRenewRelease(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()
	store := Driver(client, lock.Prefix("runa:test:"))
	ctx := context.Background()
	state, ok, err := store.Try(ctx, "a", "token-a", time.Second)
	if err != nil || !ok || state.Fencing == 0 {
		t.Fatalf("try=%#v ok=%v err=%v", state, ok, err)
	}
	if _, ok, err := store.Try(ctx, "a", "token-b", time.Second); err != nil || ok {
		t.Fatalf("second try ok=%v err=%v", ok, err)
	}
	if err := store.Renew(ctx, "a", "wrong", time.Second); !errors.Is(err, lock.ErrNotHeld) {
		t.Fatalf("wrong renew err=%v", err)
	}
	if err := store.Renew(ctx, "a", "token-a", 2*time.Second); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if err := store.Release(ctx, "a", "wrong"); !errors.Is(err, lock.ErrNotHeld) {
		t.Fatalf("wrong release err=%v", err)
	}
	if err := store.Release(ctx, "a", "token-a"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, ok, err := store.Try(ctx, "a", "token-c", time.Second); err != nil || !ok {
		t.Fatalf("try after release ok=%v err=%v", ok, err)
	}
}

func TestRedisStoreTTLExpiry(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	defer client.Close()
	store := Driver(client, lock.Prefix("runa:ttl:"))
	ctx := context.Background()
	if _, ok, err := store.Try(ctx, "a", "token-a", time.Second); err != nil || !ok {
		t.Fatalf("try ok=%v err=%v", ok, err)
	}
	server.FastForward(2 * time.Second)
	if _, ok, err := store.Try(ctx, "a", "token-b", time.Second); err != nil || !ok {
		t.Fatalf("try after ttl ok=%v err=%v", ok, err)
	}
}

func TestProviderUsesInjectedClientWithoutClosingIt(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	app := newLockTestApp(t, Provider(Client(client), Prefix("provider:lock:")))
	ctx := context.Background()
	locker := lock.Default().MustOf(lock.DefaultName)
	lease, ok, err := locker.Try(ctx, "a")
	if err != nil || !ok {
		t.Fatalf("try ok=%v err=%v", ok, err)
	}
	_ = lease.Release(ctx)
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
	app := newLockTestApp(t, Provider(Addr(server.Addr()), Prefix("provider:lock:")))
	ctx := context.Background()
	locker := lock.Default().MustOf(lock.DefaultName)
	lease, ok, err := locker.Try(ctx, "a")
	if err != nil || !ok {
		t.Fatalf("try ok=%v err=%v", ok, err)
	}
	_ = lease.Release(ctx)
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func newLockTestApp(t *testing.T, provider runaprovider.Provider) *runa.App {
	t.Helper()
	app := runa.New()
	app.Install(
		lock.Provider(lock.RegisterLocker(lock.DefaultName, lock.Use("redis"))),
		provider,
	)
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	return app
}
