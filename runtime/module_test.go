package runtime

import (
	"context"
	"reflect"
	"testing"

	runaprovider "github.com/duxweb/runa/provider"
)

type dependencyModule struct {
	ModuleBase
	name    string
	depends []string
	calls   *[]string
}

func (module dependencyModule) Name() string { return module.name }
func (module dependencyModule) Depends() []string {
	return append([]string(nil), module.depends...)
}
func (module dependencyModule) Init(context.Context, runaprovider.Context) error {
	*module.calls = append(*module.calls, module.name+":init")
	return nil
}
func (module dependencyModule) Register(context.Context, runaprovider.Context) error {
	*module.calls = append(*module.calls, module.name+":register")
	return nil
}
func (module dependencyModule) Boot(context.Context, runaprovider.Context) error {
	*module.calls = append(*module.calls, module.name+":boot")
	return nil
}
func (module dependencyModule) Shutdown(context.Context, runaprovider.Context) error {
	*module.calls = append(*module.calls, module.name+":shutdown")
	return nil
}

func TestModuleDependsSortsLifecycle(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Module(
		dependencyModule{name: "article", depends: []string{"system"}, calls: &calls},
		dependencyModule{name: "system", calls: &calls},
	)
	if err := app.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	expected := []string{
		"system:init", "article:init",
		"system:register", "article:register",
		"system:boot", "article:boot",
		"article:shutdown", "system:shutdown",
	}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestModuleDependsErrors(t *testing.T) {
	calls := []string{}
	missing := newRuntimeApp()
	missing.Module(dependencyModule{name: "article", depends: []string{"system"}, calls: &calls})
	if err := missing.Freeze(context.Background()); err == nil {
		t.Fatal("expected missing dependency error")
	}

	duplicate := newRuntimeApp()
	duplicate.Module(
		dependencyModule{name: "system", calls: &calls},
		dependencyModule{name: "system", calls: &calls},
	)
	if err := duplicate.Freeze(context.Background()); err == nil {
		t.Fatal("expected duplicate module error")
	}
}

func TestModuleInfo(t *testing.T) {
	calls := []string{}
	app := newRuntimeApp()
	app.Module(
		dependencyModule{name: "article", depends: []string{"system"}, calls: &calls},
		dependencyModule{name: "system", calls: &calls},
	)
	items := app.ModuleInfo()
	if len(items) != 2 || items[0].Name != "system" || items[1].Name != "article" || items[1].Depends[0] != "system" {
		t.Fatalf("items = %#v", items)
	}
}
