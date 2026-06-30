package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/duxweb/runa/core"
)

func TestTimezoneOptionSetsCoreLocation(t *testing.T) {
	originalLocation := core.Location()
	defer core.SetLocation(originalLocation)

	app := New(Timezone("Asia/Shanghai"))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if core.Location().String() != "Asia/Shanghai" {
		t.Fatalf("location = %s", core.Location())
	}
	if app.Timezone() != "Asia/Shanghai" {
		t.Fatalf("timezone = %q", app.Timezone())
	}
}

func TestTimezoneConfigSetsCoreLocation(t *testing.T) {
	originalLocation := core.Location()
	defer core.SetLocation(originalLocation)

	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte("timezone = 'Asia/Tokyo'\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	app := New(BasePath(root))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if core.Location().String() != "Asia/Tokyo" {
		t.Fatalf("location = %s", core.Location())
	}
}

func TestTimezoneOptionOverridesConfig(t *testing.T) {
	originalLocation := core.Location()
	defer core.SetLocation(originalLocation)

	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte("timezone = 'Asia/Tokyo'\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	app := New(BasePath(root), Timezone("Asia/Shanghai"))
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if core.Location().String() != "Asia/Shanghai" {
		t.Fatalf("location = %s", core.Location())
	}
}

func TestTimezoneEnvSetsCoreLocation(t *testing.T) {
	originalLocation := core.Location()
	defer core.SetLocation(originalLocation)

	t.Setenv("RUNA_APP_TIMEZONE", "Asia/Shanghai")

	app := New()
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if core.Location().String() != "Asia/Shanghai" {
		t.Fatalf("location = %s", core.Location())
	}
}
