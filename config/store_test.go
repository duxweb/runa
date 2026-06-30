package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testPaths struct {
	root string
}

func (paths testPaths) BasePath(items ...string) string {
	return filepath.Join(append([]string{paths.root}, items...)...)
}
func (paths testPaths) AppPath(items ...string) string {
	return paths.BasePath(append([]string{"app"}, items...)...)
}
func (paths testPaths) ConfigPath(items ...string) string {
	return paths.BasePath(append([]string{"config"}, items...)...)
}
func (paths testPaths) DataPath(items ...string) string {
	return paths.BasePath(append([]string{"data"}, items...)...)
}
func (paths testPaths) PublicPath(items ...string) string {
	return paths.BasePath(append([]string{"public"}, items...)...)
}

func TestStoreDefaultSetGetAndBind(t *testing.T) {
	store := New(t.TempDir(), testPaths{})
	if err := store.Default("app.name", "Runa"); err != nil {
		t.Fatalf("default: %v", err)
	}
	if err := store.Set("app.debug", true); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !store.Has("app.name") || store.Get[string]("app.name") != "Runa" || !store.Get[bool]("app.debug") {
		t.Fatalf("values not stored")
	}
	type appConfig struct {
		Name  string `toml:"name"`
		Debug bool   `toml:"debug"`
	}
	var cfg appConfig
	if err := store.Bind("app", &cfg); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if cfg.Name != "Runa" || !cfg.Debug {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestStoreScopeReadsAndWritesPrefixedKeys(t *testing.T) {
	store := New(t.TempDir(), testPaths{})
	queueConfig := store.Scope("queue")
	if err := queueConfig.Set("queues.default.driver", "memory"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := store.Get[string]("queue.queues.default.driver"); got != "memory" {
		t.Fatalf("root driver = %q", got)
	}
	if got := queueConfig.Get[string]("queues.default.driver"); got != "memory" {
		t.Fatalf("scoped driver = %q", got)
	}
	values := queueConfig.Values()
	if _, ok := values["queues"]; !ok {
		t.Fatalf("values = %#v", values)
	}
}

func TestStoreBindCastsStringScalars(t *testing.T) {
	store := New(t.TempDir(), testPaths{})
	if err := store.Load(Map(map[string]any{
		"app": map[string]any{
			"port":    "8080",
			"debug":   "true",
			"timeout": "30s",
		},
	})); err != nil {
		t.Fatalf("load: %v", err)
	}
	type appConfig struct {
		Port    int           `toml:"port"`
		Debug   bool          `toml:"debug"`
		Timeout time.Duration `toml:"timeout"`
	}
	var cfg appConfig
	if err := store.Bind("app", &cfg); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if cfg.Port != 8080 || !cfg.Debug || cfg.Timeout != 30*time.Second {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestStoreGetMapReturnsSnapshot(t *testing.T) {
	store := New(t.TempDir(), testPaths{})
	if err := store.Set("app.name", "Runa"); err != nil {
		t.Fatalf("set: %v", err)
	}
	values := store.Get[map[string]any]("app")
	values["name"] = "Changed"
	if got := store.Get[string]("app.name"); got != "Runa" {
		t.Fatalf("app.name = %q", got)
	}
}

func TestStoreEnvPlaceholderSelfReferenceDoesNotLoop(t *testing.T) {
	t.Setenv("RUNA_SELF", "%env(RUNA_SELF)%")
	store := New(t.TempDir(), testPaths{})
	if err := store.Set("app.self", "%env(RUNA_SELF)%"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := store.Get[string]("app.self"); got != "%env(RUNA_SELF)%" {
		t.Fatalf("self = %q", got)
	}
}

func TestStoreLoadsTOMLAndResolvesPlaceholders(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.toml"), []byte(`[app]
name = "Runa"
secret = "%env(RUNA_TEST_SECRET)%"
log = "%data_path(logs/app.log)%"
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("RUNA_TEST_SECRET", "secret")
	store := New(root, testPaths{root: root})
	if err := store.Load(File("app.toml")); err != nil {
		t.Fatalf("load: %v", err)
	}
	if store.Get[string]("app.secret") != "secret" {
		t.Fatalf("secret = %q", store.Get[string]("app.secret"))
	}
	if store.Get[string]("app.log") != filepath.Join(root, "data", "logs", "app.log") {
		t.Fatalf("log = %q", store.Get[string]("app.log"))
	}
}

func TestStoreFileDomainScopesTOML(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "queue.toml"), []byte(`[queues.default]
driver = "memory"
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	store := New(root, testPaths{root: root})
	if err := store.Load(FileDomain("queue.toml", "queue")); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := store.Get[string]("queue.queues.default.driver"); got != "memory" {
		t.Fatalf("root driver = %q", got)
	}
	if got := store.Scope("queue").Get[string]("queues.default.driver"); got != "memory" {
		t.Fatalf("scoped driver = %q", got)
	}
}

func TestEnvSourceLoadsNestedKeys(t *testing.T) {
	t.Setenv("RUNA_APP_NAME", "Runa")
	t.Setenv("RUNA_DATABASE_DEFAULT_DSN", "sqlite://memory")
	store := New(t.TempDir(), testPaths{})
	if err := store.Load(Env("RUNA_")); err != nil {
		t.Fatalf("load env: %v", err)
	}
	if store.Get[string]("app.name") != "Runa" {
		t.Fatalf("app.name = %q", store.Get[string]("app.name"))
	}
	if store.Get[string]("database.default.dsn") != "sqlite://memory" {
		t.Fatalf("dsn = %q", store.Get[string]("database.default.dsn"))
	}
}

func TestStoreLoadReturnsSourceError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.toml"), []byte(`[app`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	store := New(root, testPaths{root: root})
	if err := store.Load(File("bad.toml")); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestStoreReloadDropsRemovedSourceKeys(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app.toml")
	if err := os.WriteFile(path, []byte(`[app]
name = "Runa"
debug = true
`), 0o644); err != nil {
		t.Fatalf("write first: %v", err)
	}
	store := New(root, testPaths{root: root})
	if err := store.Default("app.page_size", 20); err != nil {
		t.Fatalf("default: %v", err)
	}
	if err := store.Set("app.env", "local"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := store.Load(File("app.toml")); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !store.Get[bool]("app.debug") {
		t.Fatal("debug not loaded")
	}
	if err := os.WriteFile(path, []byte(`[app]
name = "Runa"
`), 0o644); err != nil {
		t.Fatalf("write second: %v", err)
	}
	if err := store.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if store.Has("app.debug") {
		t.Fatal("removed source key should not persist")
	}
	if store.Get[int]("app.page_size") != 20 {
		t.Fatalf("default missing after reload")
	}
	if store.Get[string]("app.env") != "local" {
		t.Fatalf("override missing after reload")
	}
}
