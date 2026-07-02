package redis

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/duxweb/runa"
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

func TestProviderRegistersRedisRuntime(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	app := runa.New()
	app.Install(
		database.Provider(),
		Provider(Register("redis", Addr(server.Addr()), DB(1))),
	)
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	runtime := database.Default().Get("redis")
	if runtime == nil {
		t.Fatal("redis runtime is nil")
	}
	if err := runtime.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestProviderRegistersRootConfigRedisRuntime(t *testing.T) {
	server := miniredis.RunT(t)
	basePath := t.TempDir()
	configDir := filepath.Join(basePath, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	body := "addr = '" + server.Addr() + "'\ndb = 2\n"
	if err := os.WriteFile(filepath.Join(configDir, "redis.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ctx := context.Background()
	app := runa.New(runa.BasePath(basePath))
	app.Install(database.Provider(), Provider())
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	})
	runtime := database.Default().Get(database.DefaultName)
	if runtime == nil {
		t.Fatal("default redis runtime is nil")
	}
	if err := runtime.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestProviderRegistersNamedConfigRedisRuntime(t *testing.T) {
	server := miniredis.RunT(t)
	basePath := t.TempDir()
	configDir := filepath.Join(basePath, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	body := "[cache]\naddr = '" + server.Addr() + "'\ndb = 3\n"
	if err := os.WriteFile(filepath.Join(configDir, "redis.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ctx := context.Background()
	app := runa.New(runa.BasePath(basePath))
	app.Install(database.Provider(), Provider())
	if err := app.Freeze(ctx); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	t.Cleanup(func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	})
	runtime := database.Default().Get("cache")
	if runtime == nil {
		t.Fatal("cache redis runtime is nil")
	}
	if err := runtime.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestDriverUsesInjectedClientWithoutClosingIt(t *testing.T) {
	server := miniredis.RunT(t)
	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	runtime, err := Driver(Client(client), Meta("role", "shared")).Open(ctx, database.Config{Name: "shared"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := runtime.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err := runtime.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("injected client should remain open: %v", err)
	}
	_ = client.Close()
}
