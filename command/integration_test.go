package command_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/duxweb/runa"
)

func TestInspectCommands(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "app.toml"), []byte("name='runa'\nsecret='hidden'\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out bytes.Buffer
	app := runa.New(runa.BasePath(dir), runa.Writer(&out))
	if err := app.Execute(context.Background(), []string{"config:show"}); err != nil {
		t.Fatalf("config:show: %v", err)
	}
	if !strings.Contains(out.String(), "app.name") || !strings.Contains(out.String(), "runa") {
		t.Fatalf("config output = %q", out.String())
	}
	if strings.Contains(out.String(), "hidden") || !strings.Contains(out.String(), "app.secret") || !strings.Contains(out.String(), "***") {
		t.Fatalf("config secret output = %q", out.String())
	}
}

func TestConfigShowJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "app.toml"), []byte("name='runa'\nsecret='hidden'\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out bytes.Buffer
	app := runa.New(runa.BasePath(dir), runa.Writer(&out))
	if err := app.Execute(context.Background(), []string{"config:show", "--json"}); err != nil {
		t.Fatalf("config:show --json: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(out.Bytes(), &body); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	appConfig := body["app"].(map[string]any)
	if appConfig["name"] != "runa" || appConfig["secret"] != "***" {
		t.Fatalf("json body = %#v", body)
	}
}

func TestInspectAndSchemaCommands(t *testing.T) {
	var inspectOut bytes.Buffer
	app := runa.New(runa.Writer(&inspectOut))
	if err := app.Execute(context.Background(), []string{"inspect"}); err != nil {
		t.Fatalf("inspect: %v", err)
	}
	var inspect map[string]any
	if err := json.Unmarshal(inspectOut.Bytes(), &inspect); err != nil {
		t.Fatalf("inspect json: %v", err)
	}
	if inspect["env"] == "" || inspect["commands"] == nil {
		t.Fatalf("inspect = %#v", inspect)
	}

	var schemaOut bytes.Buffer
	app = runa.New(runa.Writer(&schemaOut))
	if err := app.Execute(context.Background(), []string{"schema"}); err != nil {
		t.Fatalf("schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaOut.Bytes(), &schema); err != nil {
		t.Fatalf("schema json: %v", err)
	}
	if schema["$schema"] == "" {
		t.Fatalf("schema = %#v", schema)
	}
}
