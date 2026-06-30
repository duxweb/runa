package runtime

func newRuntimeApp(options ...Option) *App {
	return New(options...)
}
