package runa

import "github.com/duxweb/runa/runtime"

// Provide registers a lazy dependency constructor on the default container.
func Provide[T any](constructor func(*App) (T, error)) {
	runtime.ProvideDefault(constructor)
}

// ProvideValue registers an eager dependency value on the default container.
func ProvideValue[T any](value T) {
	runtime.ProvideDefaultValue(value)
}

// Invoke resolves a dependency from the default container.
func Invoke[T any]() (T, error) {
	return runtime.InvokeDefault[T]()
}

// MustInvoke resolves a dependency from the default container or panics.
func MustInvoke[T any]() T {
	return runtime.MustInvokeDefault[T]()
}
