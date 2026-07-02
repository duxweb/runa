package oro

import (
	"context"
	"testing"

	orodb "github.com/duxweb/oro"
	"github.com/duxweb/runa/database"
	_ "modernc.org/sqlite"
)

func TestDatabaseOpensOroRuntime(t *testing.T) {
	driver := Driver(DSN(":memory:"), Dialect("sqlite"), Meta("role", "primary"))
	runtime, err := driver.Open(context.Background(), database.Config{Name: database.DefaultName})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		if err := runtime.Close(context.Background()); err != nil {
			t.Fatalf("close: %v", err)
		}
	}()
	if runtime.Name() != database.DefaultName {
		t.Fatalf("name = %q", runtime.Name())
	}
	if runtime.Kind() != "oro" {
		t.Fatalf("kind = %q", runtime.Kind())
	}
	if runtime.Raw() == nil {
		t.Fatal("raw db is nil")
	}
	if err := runtime.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	info := runtime.Info()
	if info.Dialect != "sqlite" || info.Meta["role"] != "primary" {
		t.Fatalf("info = %#v", info)
	}
}

func TestDriverUsesInjectedOroDBWithoutClosingIt(t *testing.T) {
	ctx := context.Background()
	base, err := Driver(DSN(":memory:"), Dialect("sqlite")).Open(ctx, database.Config{Name: "base"})
	if err != nil {
		t.Fatalf("open base: %v", err)
	}
	raw, ok := base.Raw().(*orodb.DB)
	if !ok || raw == nil {
		t.Fatalf("raw = %#v", base.Raw())
	}
	injected, err := Driver(DB(raw), Dialect("sqlite"), Meta("role", "shared")).Open(ctx, database.Config{Name: "shared"})
	if err != nil {
		t.Fatalf("open injected: %v", err)
	}
	if err := injected.Close(ctx); err != nil {
		t.Fatalf("close injected: %v", err)
	}
	if err := base.Ping(ctx); err != nil {
		t.Fatalf("injected DB should remain open: %v", err)
	}
	if err := base.Close(ctx); err != nil {
		t.Fatalf("close base: %v", err)
	}
}
