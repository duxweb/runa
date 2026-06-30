package lang

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/duxweb/runa"
)

func TestRegistryLoadDirTranslateAndPlural(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "en.toml"), `[hello]
other = "Hello {{.Name}}"

[items]
one = "{{.Count}} item"
other = "{{.Count}} items"
`)
	write(t, filepath.Join(dir, "zh.toml"), `[hello]
other = "你好 {{.Name}}"

[items]
other = "{{.Count}} 个项目"
`)
	registry := New(DefaultLocale("en"))
	if err := registry.LoadDir(dir); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := registry.Translator("zh-CN,zh;q=0.9,en;q=0.8").T("hello", "Name", "Runa"); got != "你好 Runa" {
		t.Fatalf("hello = %q", got)
	}
	if got := registry.Translator("zh-CN,zh;q=0.9,en;q=0.8").Locale(); got != "zh" {
		t.Fatalf("locale = %q", got)
	}
	if got := registry.Translator("en").T("items", "Count", 2); got != "2 items" {
		t.Fatalf("items = %q", got)
	}
}

func TestWithTranslatorAndFallbackReplace(t *testing.T) {
	translator := NewTranslator("zh")
	ctx := WithTranslator(context.Background(), translator)
	if From(ctx) != translator {
		t.Fatalf("translator not stored")
	}
	if got := translator.T("hello {name}", map[string]any{"name": "Runa"}); got != "hello Runa" {
		t.Fatalf("fallback = %q", got)
	}
	if got := Replace("hello {name}", map[string]any{"name": "Runa"}); got != "hello Runa" {
		t.Fatalf("replace = %q", got)
	}
}

func write(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestProviderLoadsConfigDirectory(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	langDir := filepath.Join(configDir, "lang")
	if err := os.MkdirAll(langDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write(t, filepath.Join(configDir, "lang.toml"), `default = "zh"
dir = "lang"
`)
	write(t, filepath.Join(langDir, "zh.toml"), `[welcome]
other = "欢迎 {{.Name}}"
`)
	app := runa.New(runa.BasePath(root), runa.ConfigPath("config"))
	app.Install(Provider())
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if got := Default().T("welcome", "Name", "Runa"); got != "欢迎 Runa" {
		t.Fatalf("welcome = %q", got)
	}
}
