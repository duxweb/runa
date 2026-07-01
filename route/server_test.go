package route

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestStartupBannerHostPrintsHTTPInfo(t *testing.T) {
	var out bytes.Buffer
	registry := New()
	server := registry.Server(ServerConfig{Addr: ":0", ShutdownTimeout: time.Second})
	unit := withStartupBanner(server, registry, BannerConfig{Writer: &out, Env: "testing"})
	if err := unit.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer unit.Stop(context.Background())

	body := out.String()
	for _, expected := range []string{"____", "Runa HTTP", "URL", "http://localhost:", "Bind", "Env", "testing", "Unit", "PID", "Routes"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("banner missing %q in:\n%s", expected, body)
		}
	}
}

func TestStartupBannerCanBeDisabled(t *testing.T) {
	var out bytes.Buffer
	enabled := false
	registry := New()
	server := registry.Server(ServerConfig{Addr: ":0", ShutdownTimeout: time.Second})
	unit := withStartupBanner(server, registry, BannerConfig{Enabled: &enabled, Writer: &out, Env: "testing"})
	if err := unit.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer unit.Stop(context.Background())
	if out.Len() != 0 {
		t.Fatalf("banner output = %q", out.String())
	}
}

func TestStartupTools(t *testing.T) {
	registry := New()
	registry.Get("/__runa", func(ctx *Context) error { return nil }).SkipDoc()
	registry.Get("/docs/openapi.json", func(ctx *Context) error { return nil }).SkipDoc()
	registry.Get("/docs", func(ctx *Context) error { return nil }).SkipDoc()
	registry.Get("/ops/health", func(ctx *Context) error { return nil }).Meta("observe", true).SkipDoc()

	tools := startupTools(registry, "http://localhost:8080")
	if len(tools) != 4 {
		t.Fatalf("tools = %#v", tools)
	}
	expected := []startupTool{
		{Name: "Console", URL: "http://localhost:8080/__runa"},
		{Name: "OpenAPI", URL: "http://localhost:8080/docs/openapi.json"},
		{Name: "Docs", URL: "http://localhost:8080/docs"},
		{Name: "Observe", URL: "http://localhost:8080/ops"},
	}
	for index, item := range expected {
		if tools[index] != item {
			t.Fatalf("tools[%d] = %#v, want %#v", index, tools[index], item)
		}
	}
}

func TestDisplayBind(t *testing.T) {
	cases := map[string]string{
		":8080":          "*:8080",
		"0.0.0.0:8080":   "*:8080",
		"[::]:8080":      "*:8080",
		"127.0.0.1:8080": "127.0.0.1:8080",
		"[::1]:8080":     "[::1]:8080",
	}
	for input, expected := range cases {
		if actual := displayBind(input); actual != expected {
			t.Fatalf("displayBind(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestDisplayHTTPURL(t *testing.T) {
	cases := map[string]string{
		":8080":           "http://localhost:8080",
		"0.0.0.0:8080":    "http://localhost:8080",
		"127.0.0.1:8080":  "http://127.0.0.1:8080",
		"[::]:8080":       "http://localhost:8080",
		"[::1]:8080":      "http://[::1]:8080",
		"example.test:80": "http://example.test:80",
	}
	for input, expected := range cases {
		if actual := displayHTTPURL(input); actual != expected {
			t.Fatalf("displayHTTPURL(%q) = %q, want %q", input, actual, expected)
		}
	}
}
