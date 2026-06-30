package devtools

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/duxweb/runa"
)

func TestProviderRegistersCommands(t *testing.T) {
	app := runa.New()
	app.Install(Provider())
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
}

func TestEmbedViewCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "views"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "views", "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write view: %v", err)
	}
	var out bytes.Buffer
	app := runa.New(runa.BasePath(dir), runa.Writer(&out))
	app.Install(Provider(Embed("views", "internal/embed/view.go", "**/*.html")))
	if err := app.Execute(context.Background(), []string{"devtools:embed"}); err != nil {
		t.Fatalf("devtools:embed: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "internal", "embed", "view.go"))
	if err != nil {
		t.Fatalf("read embed: %v", err)
	}
	if !strings.Contains(string(body), "//go:embed views/index.html") || !strings.Contains(string(body), "var ViewFS embed.FS") {
		t.Fatalf("embed body = %s", string(body))
	}
}

func TestProviderReadsConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "templates", "page.html"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write view: %v", err)
	}
	writeConfig(t, dir, "devtools.toml", `embed_root = "templates"
embed_out = "internal/embed/templates.go"
embed_name = "TemplateFS"
`)
	app := runa.New(runa.BasePath(dir))
	app.Install(Provider())
	if err := app.Execute(context.Background(), []string{"devtools:embed"}); err != nil {
		t.Fatalf("devtools:embed: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "internal", "embed", "templates.go"))
	if err != nil {
		t.Fatalf("read embed: %v", err)
	}
	if !strings.Contains(string(body), "//go:embed templates/page.html") || !strings.Contains(string(body), "var TemplateFS embed.FS") {
		t.Fatalf("embed body = %s", string(body))
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

func TestScaffold(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "app")
	if err := Scaffold(dir, "example.com/app"); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "cmd", "app", "main.go"))
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	if !strings.Contains(string(body), "Hello Runa") {
		t.Fatalf("main = %s", string(body))
	}
	guide, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(guide), "runa gen module <name>") {
		t.Fatalf("CLAUDE.md = %s", string(guide))
	}
	if err := Scaffold(dir, "example.com/app"); err == nil {
		t.Fatal("expected existing file error")
	}
}
