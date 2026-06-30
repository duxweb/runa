package runtime

import "io"

// Option configures an App during creation.
type Option func(*App)

// BasePath sets the application base path.
func BasePath(path string) Option {
	return func(app *App) {
		app.basePath = path
	}
}

// AppPath sets the application app path.
func AppPath(path string) Option {
	return func(app *App) {
		app.appPath = path
	}
}

// ConfigPath sets the application config path.
func ConfigPath(path string) Option {
	return func(app *App) {
		app.configPath = path
	}
}

// DataPath sets the application data path.
func DataPath(path string) Option {
	return func(app *App) {
		app.dataPath = path
	}
}

// PublicPath sets the application public path.
func PublicPath(path string) Option {
	return func(app *App) {
		app.publicPath = path
	}
}

// Env sets the application environment name.
func Env(name string) Option {
	return func(app *App) {
		app.env = name
	}
}

// Timezone sets the application timezone.
func Timezone(name string) Option {
	return func(app *App) {
		app.timezone = name
	}
}

// Writer sets command output writers.
func Writer(out io.Writer, errOut ...io.Writer) Option {
	return func(app *App) {
		if out != nil {
			app.writer = out
		}
		if len(errOut) > 0 && errOut[0] != nil {
			app.errWriter = errOut[0]
		}
	}
}
