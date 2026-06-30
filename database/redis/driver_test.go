package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa/database"
	goredis "github.com/redis/go-redis/v9"
)

func TestDatabaseOpensRedisRuntime(t *testing.T) {
	server := miniredis.RunT(t)
	driver := Driver(Addr(server.Addr()), DB(1), Meta("role", "cache"))
	runtime, err := driver.Open(context.Background(), database.Config{Name: "cache"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		if err := runtime.Close(context.Background()); err != nil {
			t.Fatalf("close: %v", err)
		}
	}()
	if runtime.Name() != "cache" {
		t.Fatalf("name = %q", runtime.Name())
	}
	if runtime.Kind() != "redis" {
		t.Fatalf("kind = %q", runtime.Kind())
	}
	client, ok := runtime.Raw().(*goredis.Client)
	if !ok || client == nil {
		t.Fatalf("raw = %#v", runtime.Raw())
	}
	if err := runtime.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	info := runtime.Info()
	if info.Dialect != "redis" || info.Meta["role"] != "cache" {
		t.Fatalf("info = %#v", info)
	}
}
