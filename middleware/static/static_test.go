package static

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/duxweb/runa/route"
)

func TestStaticServesFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.txt"), []byte("asset"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Root: http.Dir(dir), Path: "/assets", Index: true}))
	group.Get("/assets/app.txt", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("handler") })

	request := httptest.NewRequest(http.MethodGet, "/assets/app.txt", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.String() != "asset" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestStaticFallsThroughMissingFile(t *testing.T) {
	dir := t.TempDir()
	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Root: http.Dir(dir), Path: "/assets", Index: true}))
	group.Get("/assets/missing.txt", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("handler") })

	request := httptest.NewRequest(http.MethodGet, "/assets/missing.txt", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "handler" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestStaticNextSkips(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.txt"), []byte("asset"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Root: http.Dir(dir), Path: "/assets", Index: true, Next: func(*route.Context) bool { return true }}))
	group.Get("/assets/app.txt", func(ctx *route.Context) error { return ctx.Status(http.StatusOK).Text("handler") })

	request := httptest.NewRequest(http.MethodGet, "/assets/app.txt", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Body.String() != "handler" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestStaticDirectoryIndexDisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.txt"), []byte("asset"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	registry := route.New()
	group := route.NewGroup(registry, "")
	group.Use(New(Config{Root: http.Dir(dir), Path: "/assets"}))

	request := httptest.NewRequest(http.MethodGet, "/assets/", nil)
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
}
