package oro

import (
	"context"
	"testing"

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
