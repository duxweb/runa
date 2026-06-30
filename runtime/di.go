package runtime

import "github.com/samber/do/v2"

// Provide registers a lazy dependency constructor.
func Provide[T any](app *App, constructor func(*App) (T, error)) {
	do.Provide(app.container, func(do.Injector) (T, error) {
		return constructor(app)
	})
}

// ProvideDefault registers a lazy dependency constructor on the default container.
func ProvideDefault[T any](constructor func(*App) (T, error)) {
	app := Default()
	Provide(app, constructor)
}

// ProvideValue registers an eager dependency value.
func ProvideValue[T any](app *App, value T) {
	do.ProvideValue(app.container, value)
}

// ProvideDefaultValue registers an eager dependency value on the default container.
func ProvideDefaultValue[T any](value T) {
	ProvideValue(Default(), value)
}

// Invoke returns a dependency from the application container.
func Invoke[T any](app *App) (T, error) {
	return do.Invoke[T](app.container)
}

// InvokeDefault returns a dependency from the default container.
func InvokeDefault[T any]() (T, error) {
	return do.Invoke[T](DefaultInjector())
}

// MustInvoke returns a dependency or panics.
func MustInvoke[T any](app *App) T {
	return do.MustInvoke[T](app.container)
}

// MustInvokeDefault returns a dependency from the default container or panics.
func MustInvokeDefault[T any]() T {
	return do.MustInvoke[T](DefaultInjector())
}
