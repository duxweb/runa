package cluster_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/duxweb/runa"
	"github.com/duxweb/runa/cluster"
)

func TestRegistryRegistersHeartbeatAndUnregisters(t *testing.T) {
	driver := cluster.MemoryDriver()
	app := runa.New()
	app.Install(cluster.Provider(
		cluster.DriverWith(driver),
		cluster.ID("node-1"),
		cluster.Service("admin"),
		cluster.HeartbeatInterval(time.Millisecond),
		cluster.TTL(time.Second),
	))
	if cluster.RegistryOf(app) != nil {
		t.Fatal("cluster should not be available before freeze")
	}
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	runtime := cluster.RegistryOf(app)
	if runtime == nil {
		t.Fatal("missing runtime")
	}
	items, err := runtime.Instances(context.Background())
	if err != nil {
		t.Fatalf("instances: %v", err)
	}
	if len(items) != 1 || items[0].ID != "node-1" || items[0].Service != "admin" {
		t.Fatalf("instances = %#v", items)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	items, err = driver.Instances(context.Background(), "admin")
	if err != nil {
		t.Fatalf("instances after shutdown: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected unregistered instance, got %#v", items)
	}
}

func TestProviderReadsConfig(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "cluster.toml", `id = "configured-node"
service = "configured"
`)
	driver := cluster.MemoryDriver()
	app := runa.New(runa.BasePath(root))
	app.Install(cluster.Provider(cluster.DriverWith(driver)))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	runtime := cluster.RegistryOf(app)
	if runtime == nil {
		t.Fatal("missing runtime")
	}
	instance := runtime.Instance()
	if instance.ID != "configured-node" || instance.Service != "configured" {
		t.Fatalf("instance = %#v", instance)
	}
}

func TestProviderOptionsOverrideConfig(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "cluster.toml", `service = "configured"
`)
	driver := cluster.MemoryDriver()
	app := runa.New(runa.BasePath(root))
	app.Install(cluster.Provider(cluster.DriverWith(driver), cluster.Service("code")))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if service := cluster.RegistryOf(app).Instance().Service; service != "code" {
		t.Fatalf("service = %q", service)
	}
}

func writeConfig(t *testing.T, root string, name string, body string) {
	t.Helper()
	dir := filepath.Join(root, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
