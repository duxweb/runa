package runtime

import (
	"path/filepath"
	"testing"
)

func TestAppPaths(t *testing.T) {
	root := t.TempDir()
	app := newRuntimeApp(BasePath(root), AppPath("src/app"), ConfigPath("settings"), DataPath("runtime"), PublicPath("www"))
	if app.AppPath("article") != filepath.Join(root, "src/app", "article") {
		t.Fatalf("app path = %q", app.AppPath("article"))
	}
	if app.ConfigPath("app.toml") != filepath.Join(root, "settings", "app.toml") {
		t.Fatalf("config path = %q", app.ConfigPath("app.toml"))
	}
	if app.DataPath("logs/app.log") != filepath.Join(root, "runtime", "logs/app.log") {
		t.Fatalf("data path = %q", app.DataPath("logs/app.log"))
	}
	if app.PublicPath("assets/app.css") != filepath.Join(root, "www", "assets/app.css") {
		t.Fatalf("public path = %q", app.PublicPath("assets/app.css"))
	}
}
