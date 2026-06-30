package view

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type testContext struct{}

func (testContext) Context() context.Context { return context.Background() }

func TestHTMLRendererDirAndReload(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pages"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	file := filepath.Join(dir, "pages", "home.html")
	if err := os.WriteFile(file, []byte("Hello {{.Name}}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	renderer := HTML(Dir(dir, "**/*.html").Reload(true))
	if err := renderer.Load(context.Background(), &Set{Name: "web", Sources: []Source{Dir(dir, "**/*.html").Reload(true)}}); err != nil {
		t.Fatalf("load: %v", err)
	}
	var buffer bytes.Buffer
	if err := renderer.Render(testContext{}, &buffer, "pages/home", map[string]any{"Name": "Runa"}); err != nil {
		t.Fatalf("render: %v", err)
	}
	if buffer.String() != "Hello Runa" {
		t.Fatalf("body = %q", buffer.String())
	}
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(file, []byte("Hi {{.Name}}"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	buffer.Reset()
	if err := renderer.Render(testContext{}, &buffer, "pages/home.html", map[string]any{"Name": "Runa"}); err != nil {
		t.Fatalf("render reload: %v", err)
	}
	if buffer.String() != "Hi Runa" {
		t.Fatalf("reloaded body = %q", buffer.String())
	}
}

func TestHTMLRendererEmbedSource(t *testing.T) {
	fsys := fstest.MapFS{
		"views/mail/welcome.tmpl": {Data: []byte("Welcome {{.Name}}")},
	}
	renderer := HTML(Embed(fsys, "views/mail", "**/*.tmpl"))
	var buffer bytes.Buffer
	if err := renderer.Render(testContext{}, &buffer, "welcome", map[string]any{"Name": "Dux"}); err != nil {
		t.Fatalf("render: %v", err)
	}
	if buffer.String() != "Welcome Dux" {
		t.Fatalf("body = %q", buffer.String())
	}
}

func TestSourcePatternBrace(t *testing.T) {
	fsys := fstest.MapFS{
		"views/a.html": {Data: []byte("a")},
		"views/b.tpl":  {Data: []byte("b")},
		"views/c.txt":  {Data: []byte("c")},
	}
	files, err := Files(Embed(fsys, "views", "**/*.{html,tpl}"))
	if err != nil {
		t.Fatalf("files: %v", err)
	}
	names := []string{}
	for _, file := range files {
		names = append(names, file.Name)
	}
	if strings.Join(names, ",") != "a.html,b.tpl" {
		t.Fatalf("names = %#v", names)
	}
}

func TestHTMLRendererContextFunc(t *testing.T) {
	fsy := fstest.MapFS{
		"views/page.html": {Data: []byte(`{{ t "hello" }}`)},
	}
	registry := New()
	registry.ContextFunc("t", func(ctx context.Context) any {
		locale, _ := ctx.Value("locale").(string)
		return func(key string) string { return locale + ":" + key }
	})
	if err := registry.Register(context.Background(), "web", HTML(Embed(fsy, "views", "**/*.html"))); err != nil {
		t.Fatalf("register: %v", err)
	}
	ctx := context.WithValue(context.Background(), "locale", "zh")
	body, err := registry.RenderString(contextWrapper{ctx: ctx}, "web", "page", nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if body != "zh:hello" {
		t.Fatalf("body = %q", body)
	}
}

type contextWrapper struct{ ctx context.Context }

func (wrapper contextWrapper) Context() context.Context { return wrapper.ctx }
