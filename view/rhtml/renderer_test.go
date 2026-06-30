package rhtml

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/duxweb/runa/view"
)

type testContext struct{}

func (testContext) Context() context.Context { return context.Background() }

func TestRendererLayoutIncludeIfForAndFuncs(t *testing.T) {
	fsys := fstest.MapFS{
		"views/layouts/admin.html": {Data: []byte(`<html><title>{{ .title }}</title><r:include name="partials/nav" label="Main" title=".title" root=".root" /><r:block name="content" user=".user" items=".items">fallback</r:block></html>`)},
		"views/partials/nav.html":  {Data: []byte(`<nav>{{ .label }}:{{ .title }}:{{ .root.Title }}</nav>`)},
		"views/pages/home.html":    {Data: []byte(`<r:layout name="admin" title=".Title" user=".User" items=".Items"><r:section name="content"><h1>{{ upper .root.Title }}</h1><r:if cond=".user"><p>{{ .user }}</p><r:else><p>guest</p></r:if><r:for value=".items" as="item"><span>{{ item.Name }}</span></r:for></r:section></r:layout>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html")).Func("upper", func(value string) string { return "[" + value + "]" })
	body := render(t, renderer, "pages/home", map[string]any{
		"Title": "Runa",
		"User":  "dux",
		"Items": []map[string]any{{"Name": "A"}, {"Name": "B"}},
	})
	want := `<html><title>Runa</title><nav>Main:Runa:Runa</nav><h1>[Runa]</h1><p>dux</p><span>A</span><span>B</span></html>`
	if body != want {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererNestedLoops(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:for value=".Groups" as="group"><r:for value="group.Items" as="item">{{ group.Name }}={{ .item }}</r:for></r:for>`)},
	}
	body := render(t, New(view.Embed(fsys, "views", "**/*.html")), "page", map[string]any{
		"Groups": []map[string]any{
			{"Name": "A", "Items": []string{"1", "2"}},
			{"Name": "B", "Items": []string{"3"}},
		},
	})
	if body != `A=1A=2B=3` {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererBlockFallbackAndElse(t *testing.T) {
	fsys := fstest.MapFS{
		"views/layouts/admin.html": {Data: []byte(`<main><r:block name="content" user=".user">fallback</r:block><r:block name="sidebar">side</r:block></main>`)},
		"views/pages/home.html":    {Data: []byte(`<r:layout name="admin" user=".User"><r:section name="content"><r:if cond=".user">user<r:else>guest</r:if></r:section></r:layout>`)},
	}
	body := render(t, New(view.Embed(fsys, "views", "**/*.html")), "pages/home", map[string]any{"User": ""})
	if body != `<main>guestside</main>` {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererNestedLayouts(t *testing.T) {
	fsys := fstest.MapFS{
		"views/layouts/base.html":  {Data: []byte(`<html><header><r:block name="header">base</r:block></header><body><r:block name="body"></r:block></body></html>`)},
		"views/layouts/admin.html": {Data: []byte(`<r:layout name="base"><r:section name="header">admin</r:section><r:section name="body"><aside><r:block name="sidebar">menu</r:block></aside><main><r:block name="content"></r:block></main></r:section></r:layout>`)},
		"views/pages/home.html":    {Data: []byte(`<r:layout name="admin"><r:section name="content">home</r:section><r:section name="sidebar">custom</r:section></r:layout>`)},
	}
	body := render(t, New(view.Embed(fsys, "views", "**/*.html")), "pages/home", nil)
	want := `<html><header>admin</header><body><aside>custom</aside><main>home</main></body></html>`
	if body != want {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererBlockSectionPropsAreIsolated(t *testing.T) {
	fsys := fstest.MapFS{
		"views/layouts/admin.html": {Data: []byte(`<main><r:block name="content" title="Layout" tone="quiet"></r:block></main>`)},
		"views/pages/home.html":    {Data: []byte(`<r:layout name="admin"><r:section name="content" title="Page">{{ .title }}:{{ .tone }}:{{ .root.Global }}</r:section></r:layout>`)},
	}
	body := render(t, New(view.Embed(fsys, "views", "**/*.html")), "pages/home", map[string]any{"Global": "root"})
	if body != `<main>Page:quiet:root</main>` {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererIncludeDataIsIsolated(t *testing.T) {
	fsys := fstest.MapFS{
		"views/partial.html": {Data: []byte(`{{ .title }}:{{ .root.Title }}`)},
		"views/page.html":    {Data: []byte(`<r:include name="partial" title="Local" />`)},
	}
	body := render(t, New(view.Embed(fsys, "views", "**/*.html")), "page", map[string]any{"Title": "Root"})
	if body != `Local:Root` {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererRejectsImplicitParentData(t *testing.T) {
	fsys := fstest.MapFS{
		"views/partial.html": {Data: []byte(`{{ .Title }}`)},
		"views/page.html":    {Data: []byte(`<r:include name="partial" />`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", map[string]any{"Title": "Root"})
	if err == nil || !strings.Contains(err.Error(), "map has no entry for key") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererUsesHTMLTemplateEscaping(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<p>{{ .HTML }}</p>`)},
	}
	body := render(t, New(view.Embed(fsys, "views", "**/*.html")), "page", map[string]any{"HTML": `<script>alert(1)</script>`})
	if body != `<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>` {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererReload(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page.html"), []byte(`one`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	renderer := New(view.Dir(dir, "**/*.html").Reload(true))
	if body := render(t, renderer, "page", nil); body != "one" {
		t.Fatalf("body = %q", body)
	}
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "page.html"), []byte(`two`), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if body := render(t, renderer, "page", nil); body != "two" {
		t.Fatalf("reloaded body = %q", body)
	}
}

func TestRendererReportsTemplateErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:include />`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", nil)
	if err == nil || !strings.Contains(err.Error(), "include name is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererRejectsPairedInclude(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:include name="partial"></r:include>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", nil)
	if err == nil || !strings.Contains(err.Error(), "self closing") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererRejectsLayoutWithOutsideContent(t *testing.T) {
	fsys := fstest.MapFS{
		"views/layouts/admin.html": {Data: []byte(`<r:block name="content"></r:block>`)},
		"views/page.html":          {Data: []byte(`<r:layout name="admin"><r:section name="content">ok</r:section></r:layout><p>outside</p>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", nil)
	if err == nil || !strings.Contains(err.Error(), "layout must wrap the whole template") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererRejectsLayoutBodyWithoutSections(t *testing.T) {
	fsys := fstest.MapFS{
		"views/layouts/admin.html": {Data: []byte(`<r:block name="content"></r:block>`)},
		"views/page.html":          {Data: []byte(`<r:layout name="admin"><p>loose</p><r:section name="content">ok</r:section></r:layout>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", nil)
	if err == nil || !strings.Contains(err.Error(), "layout can only contain sections") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererRejectsDuplicateSections(t *testing.T) {
	fsys := fstest.MapFS{
		"views/layouts/admin.html": {Data: []byte(`<r:block name="content"></r:block>`)},
		"views/page.html":          {Data: []byte(`<r:layout name="admin"><r:section name="content">one</r:section><r:section name="content">two</r:section></r:layout>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", nil)
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererRejectsSectionOutsideLayout(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:section name="content">orphan</r:section>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", nil)
	if err == nil || !strings.Contains(err.Error(), "section must be inside layout") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererRejectsInvalidForAlias(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:for value=".Items" as="bad-name"></r:for>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", map[string]any{"Items": []string{"a"}})
	if err == nil || !strings.Contains(err.Error(), "valid identifier") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererCustomTagBlockAndSelfClosing(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:article class_id=".Class.ID" status="1" as="item">{{ .item.ID }}:{{ .class_id }}:{{ .root.Site }};</r:article><r:count status="1" />`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	renderer.Tag("article", func(ctx context.Context, props Props) (any, error) {
		if props.Int("class_id") != 9 || props.Int("status") != 1 {
			t.Fatalf("props = %#v", props.Map())
		}
		return []map[string]any{{"ID": 1}, {"ID": 2}}, nil
	})
	renderer.Tag("count", func(ctx context.Context, props Props) (any, error) {
		return props.Int("status") + 2, nil
	})
	body := render(t, renderer, "page", map[string]any{"Class": map[string]any{"ID": 9}, "Site": "Runa"})
	if body != `1:9:Runa;2:9:Runa;3` {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererCustomTagSingleValueRendersOnce(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:user as="user">{{ .user.Name }}</r:user>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	renderer.Tag("user", func(ctx context.Context, props Props) (any, error) {
		return map[string]any{"Name": "Dux"}, nil
	})
	body := render(t, renderer, "page", nil)
	if body != `Dux` {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererCustomTagSelfClosingAssignsScope(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:articles status="1" as="articles" /><r:for value=".articles" as="item">{{ .item.ID }};</r:for>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	renderer.Tag("articles", func(ctx context.Context, props Props) (any, error) {
		if props.Int("status") != 1 {
			t.Fatalf("props = %#v", props.Map())
		}
		return []map[string]any{{"ID": 1}, {"ID": 2}}, nil
	})
	body := render(t, renderer, "page", nil)
	if body != `1;2;` {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererCustomTagSelfClosingAssignsNestedScopes(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:site as="site" /><r:articles site_id=".site.ID" as="articles" />{{ .site.Name }}:<r:for value=".articles" as="item">{{ .item.ID }}</r:for>`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	renderer.Tag("site", func(ctx context.Context, props Props) (any, error) {
		return map[string]any{"ID": 7, "Name": "Runa"}, nil
	})
	renderer.Tag("articles", func(ctx context.Context, props Props) (any, error) {
		if props.Int("site_id") != 7 {
			t.Fatalf("props = %#v", props.Map())
		}
		return []map[string]any{{"ID": 1}, {"ID": 2}}, nil
	})
	body := render(t, renderer, "page", nil)
	if body != `Runa:12` {
		t.Fatalf("body = %q", body)
	}
}

func TestRendererCustomTagRejectsInvalidAssignAlias(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:articles as="bad-name" />`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	renderer.Tag("articles", func(ctx context.Context, props Props) (any, error) { return nil, nil })
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", nil)
	if err == nil || !strings.Contains(err.Error(), "valid identifier") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererCustomTagErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"views/page.html": {Data: []byte(`<r:missing />`)},
	}
	renderer := New(view.Embed(fsys, "views", "**/*.html"))
	var buffer bytes.Buffer
	err := renderer.Render(testContext{}, &buffer, "page", nil)
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("err = %v", err)
	}
}

func TestRendererRejectsReservedCustomTag(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	New().Tag("for", func(context.Context, Props) (any, error) { return nil, nil })
}

func TestRendererViewSet(t *testing.T) {
	source := view.Embed(fstest.MapFS{"views/page.html": {Data: []byte(`ok`)}}, "views", "**/*.html")
	renderer := New(source)
	set := renderer.ViewSet()
	if len(set.Sources) != 1 {
		t.Fatalf("sources = %d", len(set.Sources))
	}
}

func render(t *testing.T, renderer *Renderer, name string, data any) string {
	t.Helper()
	var buffer bytes.Buffer
	if err := renderer.Render(testContext{}, &buffer, name, data); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	return buffer.String()
}

func TestRendererContextFunc(t *testing.T) {
	fsy := fstest.MapFS{
		"views/page.html": {Data: []byte(`{{ t "hello" }}`)},
	}
	registry := view.New()
	registry.ContextFunc("t", func(ctx context.Context) any {
		locale, _ := ctx.Value("locale").(string)
		return func(key string) string { return locale + ":" + key }
	})
	if err := registry.Register(context.Background(), "web", New(view.Embed(fsy, "views", "**/*.html"))); err != nil {
		t.Fatalf("register: %v", err)
	}
	ctx := context.WithValue(context.Background(), "locale", "en")
	body, err := registry.RenderString(rhtmlContextWrapper{ctx: ctx}, "web", "page", nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if body != "en:hello" {
		t.Fatalf("body = %q", body)
	}
}

type rhtmlContextWrapper struct{ ctx context.Context }

func (wrapper rhtmlContextWrapper) Context() context.Context { return wrapper.ctx }
