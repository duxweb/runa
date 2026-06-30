package view

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/duxweb/runa"
	runalang "github.com/duxweb/runa/lang"
	"github.com/duxweb/runa/provider"
	runaview "github.com/duxweb/runa/view"
	"github.com/duxweb/runa/view/rhtml"
)

type testContext struct{ ctx context.Context }

func (ctx testContext) Context() context.Context { return ctx.ctx }

type testModule struct{ provider.ModuleBase }

func (testModule) Name() string { return "test" }

func (testModule) Register(ctx context.Context, app provider.Context) error {
	views, err := provider.Invoke[*runaview.Registry](app)
	if err != nil {
		return err
	}
	fsy := fstest.MapFS{"views/page.html": {Data: []byte(`{{ t "hello {Name}" "Name" .Name }}`)}}
	return views.Register(ctx, "web", rhtml.New(runaview.Embed(fsy, "views", "**/*.html")))
}

func TestProviderInjectsT(t *testing.T) {
	app := runa.New()
	app.Install(runalang.Provider(runalang.DefaultLocale("en")), runaview.Provider(), Provider())
	app.Module(testModule{})
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	views := runaview.Default()
	renderCtx := testContext{ctx: runalang.WithTranslator(context.Background(), runalang.Default().Translator("en"))}
	body, err := views.RenderString(renderCtx, "web", "page", map[string]any{"Name": "Runa"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if body != "hello Runa" {
		t.Fatalf("body = %q", body)
	}
}
