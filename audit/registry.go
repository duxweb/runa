package audit

// Registry stores normalized audit configuration.
type Registry struct {
	config Config
}

// New creates an audit registry from config.
func New(config Config) *Registry {
	config = Normalize(config)
	if config.Writer == nil && config.Write != nil {
		config.Writer = FuncWriter(config.Write)
	}
	return &Registry{config: config}
}

// Config returns the normalized audit config.
func (registry *Registry) Config() Config { return registry.config }
