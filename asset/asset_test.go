package asset

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/duxweb/runa/view"
)

func TestAssetURLManifestAndHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"public/app.css":        {Data: []byte("body{}")},
		"public/app.abc123.css": {Data: []byte("body{}")},
		"public/manifest.json":  {Data: []byte(`{"app.css":"app.abc123.css"}`)},
	}
	set := Assets(view.Embed(fsys, "public", "**/*.{css,json}")).Prefix("/assets/web").Manifest("manifest.json")
	if err := set.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	if url := set.URL("app.css"); url != "/assets/web/app.abc123.css" {
		t.Fatalf("url = %q", url)
	}
	request := httptest.NewRequest(http.MethodGet, "/assets/web/app.css", nil)
	response := httptest.NewRecorder()
	set.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != "body{}" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("ETag") == "" || response.Header().Get("Last-Modified") == "" {
		t.Fatalf("headers = %#v", response.Header())
	}
}

func TestAssetRegistry(t *testing.T) {
	fsys := fstest.MapFS{"public/app.js": {Data: []byte("console.log(1)")}}
	registry := New()
	if err := registry.Register(context.Background(), "web", Assets(view.Embed(fsys, "public", "**/*.js")).Prefix("/static")); err != nil {
		t.Fatalf("register: %v", err)
	}
	if url := registry.URL("web", "app.js"); !strings.HasPrefix(url, "/static/app.js?v=") {
		t.Fatalf("url = %q", url)
	}
	if info := registry.Info(); len(info) != 1 || info[0].Name != "web" || info[0].Files != 1 {
		t.Fatalf("info = %#v", info)
	}
}
