package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

// BasePath returns the application base path joined with optional segments.
func (app *App) BasePath(paths ...string) string {
	app.mu.Lock()
	base := app.basePath
	app.mu.Unlock()
	return joinBase(base, paths...)
}

// AppPath returns the application app path joined with optional segments.
func (app *App) AppPath(paths ...string) string {
	return app.BasePath(append([]string{app.appPath}, paths...)...)
}

// ConfigPath returns the application config path joined with optional segments.
func (app *App) ConfigPath(paths ...string) string {
	return app.BasePath(append([]string{app.configPath}, paths...)...)
}

// DataPath returns the application data path joined with optional segments.
func (app *App) DataPath(paths ...string) string {
	return app.BasePath(append([]string{app.dataPath}, paths...)...)
}

// PublicPath returns the application public path joined with optional segments.
func (app *App) PublicPath(paths ...string) string {
	return app.BasePath(append([]string{app.publicPath}, paths...)...)
}

func joinBase(base string, paths ...string) string {
	if base == "" {
		base = "."
	}
	for _, item := range paths {
		if filepath.IsAbs(item) {
			return filepath.Clean(filepath.Join(paths...))
		}
	}
	items := append([]string{base}, paths...)
	return filepath.Clean(filepath.Join(items...))
}

func defaultBasePath() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func defaultEnv() string {
	for _, name := range []string{"RUNA_ENV", "APP_ENV"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return "local"
}
