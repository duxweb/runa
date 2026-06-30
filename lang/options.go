package lang

type optionConfig struct {
	defaultLocale string
}

// Option configures the language registry.
type Option func(*optionConfig)

// DefaultLocale sets the fallback locale.
func DefaultLocale(locale string) Option {
	return func(config *optionConfig) {
		config.defaultLocale = locale
	}
}
