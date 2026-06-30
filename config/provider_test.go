package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runacommand "github.com/duxweb/runa/command"
	"github.com/duxweb/runa/host"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/samber/do/v2"
	cli "github.com/urfave/cli/v3"
)

type providerContext struct {
	injector do.Injector
	commands []runacommand.Command
}

func (ctx *providerContext) App() any              { return nil }
func (ctx *providerContext) Injector() do.Injector { return ctx.injector }
func (ctx *providerContext) RegisterCommand(commands ...runacommand.Command) error {
	ctx.commands = append(ctx.commands, commands...)
	return nil
}
func (ctx *providerContext) RegisterService(...any) error    { return nil }
func (ctx *providerContext) RegisterModule(...any) error     { return nil }
func (ctx *providerContext) RegisterHost(...host.Unit) error { return nil }
func (ctx *providerContext) RegisterRouteService(...any) error {
	return nil
}

var _ runaprovider.Context = (*providerContext)(nil)

func TestProviderLoadsFilesEnvAndDefaults(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeConfig(t, configDir, "app.toml", `name = "base"
debug = false
`)
	writeConfig(t, configDir, "app.local.toml", `name = "local"
debug = true
`)
	writeConfig(t, configDir, "app.prod.toml", `name = "prod"
debug = false
`)

	injector := do.New()
	ctx := &providerContext{injector: injector}
	provider := Provider(root, testPaths{root: root}, configDir, "local", "RUNA_")
	if err := provider.Init(t.Context(), ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	store := do.MustInvoke[*Store](injector)
	if err := store.Default("article.page_size", 20); err != nil {
		t.Fatalf("default page: %v", err)
	}
	if err := store.Default("app.name", "default"); err != nil {
		t.Fatalf("default name: %v", err)
	}
	if err := store.Default("app.debug", false); err != nil {
		t.Fatalf("default debug: %v", err)
	}
	if err := provider.Register(ctx); err != nil {
		t.Fatalf("register: %v", err)
	}
	if store.Get[string]("app.name") != "local" || !store.Get[bool]("app.debug") {
		t.Fatalf("config app = %q %v", store.Get[string]("app.name"), store.Get[bool]("app.debug"))
	}
	if store.Get[int]("article.page_size") != 20 {
		t.Fatalf("module default = %d", store.Get[int]("article.page_size"))
	}
	for _, path := range []string{"cache", "logs", "tmp", "uploads"} {
		if _, err := os.Stat(filepath.Join(root, "data", path)); err != nil {
			t.Fatalf("runtime dir %s: %v", path, err)
		}
	}
	if len(ctx.commands) != 1 {
		t.Fatalf("commands = %#v", ctx.commands)
	}
}

func TestProviderEnvOverridesFiles(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeConfig(t, configDir, "app.toml", "name = 'file'\n")
	t.Setenv("RUNA_APP_NAME", "env")

	injector := do.New()
	ctx := &providerContext{injector: injector}
	provider := Provider(root, testPaths{root: root}, configDir, "local", "RUNA_")
	if err := provider.Init(t.Context(), ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	store := do.MustInvoke[*Store](injector)
	if err := store.Default("app.name", "default"); err != nil {
		t.Fatalf("default: %v", err)
	}
	if err := provider.Register(ctx); err != nil {
		t.Fatalf("register: %v", err)
	}
	if store.Get[string]("app.name") != "env" {
		t.Fatalf("app.name = %q", store.Get[string]("app.name"))
	}
}

func TestProviderUsesDefaultsWithoutFiles(t *testing.T) {
	root := t.TempDir()
	injector := do.New()
	ctx := &providerContext{injector: injector}
	provider := Provider(root, testPaths{root: root}, filepath.Join(root, "config"), "local", "RUNA_")
	if err := provider.Init(t.Context(), ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	store := do.MustInvoke[*Store](injector)
	if err := store.Default("app.name", "default"); err != nil {
		t.Fatalf("default name: %v", err)
	}
	if err := store.Default("app.debug", true); err != nil {
		t.Fatalf("default debug: %v", err)
	}
	if err := provider.Register(ctx); err != nil {
		t.Fatalf("register: %v", err)
	}
	if store.Get[string]("app.name") != "default" || !store.Get[bool]("app.debug") {
		t.Fatalf("defaults = %q %v", store.Get[string]("app.name"), store.Get[bool]("app.debug"))
	}
}

func TestProviderDoesNotGenerateConfigDirectory(t *testing.T) {
	root := t.TempDir()
	injector := do.New()
	ctx := &providerContext{injector: injector}
	provider := Provider(root, testPaths{root: root}, filepath.Join(root, "config"), "local", "RUNA_")
	if err := provider.Init(t.Context(), ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := provider.Register(ctx); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "config")); err == nil {
		t.Fatal("config directory should not be generated")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat config: %v", err)
	}
}

func TestShowCommandMasksSensitiveValues(t *testing.T) {
	store := New(t.TempDir(), testPaths{})
	if err := store.Set("app.secret", "plain-value"); err != nil {
		t.Fatalf("set secret: %v", err)
	}
	if err := store.Set("app.name", "runa"); err != nil {
		t.Fatalf("set name: %v", err)
	}
	var output bytes.Buffer
	root := &cli.Command{Writer: &output}
	cliCommand := &cli.Command{Writer: &output}
	cliCommand.Root().Writer = root.Writer
	if err := (showCommand{store: store}).Run(t.Context(), runacommand.NewContext(nil, cliCommand)); err != nil {
		t.Fatalf("run: %v", err)
	}
	body := output.String()
	if strings.Contains(body, "plain-value") || !strings.Contains(body, "***") || !strings.Contains(body, "runa") {
		t.Fatalf("output = %q", body)
	}
}

func writeConfig(t *testing.T, dir string, name string, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
