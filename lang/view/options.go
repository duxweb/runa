package view

type optionConfig struct {
	name string
}

// Option configures the lang-view integration.
type Option func(*optionConfig)

// FuncName changes the injected template function name.
func FuncName(name string) Option {
	return func(config *optionConfig) {
		config.name = name
	}
}
