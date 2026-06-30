package devtools

// Config configures devtools commands.
type Config struct {
	EmbedRoot     string   `toml:"embed_root"`
	EmbedPatterns []string `toml:"embed_patterns"`
	EmbedPackage  string   `toml:"embed_package"`
	EmbedName     string   `toml:"embed_name"`
	EmbedOut      string   `toml:"embed_out"`
}

// Option configures devtools provider.
type Option func(*Config)

func defaultConfig() Config {
	return Config{EmbedRoot: "views", EmbedPatterns: []string{"**/*.{html,tmpl,tpl}"}, EmbedPackage: "embed", EmbedName: "ViewFS", EmbedOut: "internal/embed/view.go"}
}

// Embed configures embed generation defaults.
func Embed(root string, out string, patterns ...string) Option {
	return func(config *Config) {
		if root != "" {
			config.EmbedRoot = root
		}
		if out != "" {
			config.EmbedOut = out
		}
		if len(patterns) > 0 {
			config.EmbedPatterns = append([]string(nil), patterns...)
		}
	}
}
