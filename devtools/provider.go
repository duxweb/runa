package devtools

import (
	runaconfig "github.com/duxweb/runa/config"
	runaprovider "github.com/duxweb/runa/provider"
)

// Provider registers development helper commands.
func Provider(options ...Option) runaprovider.Provider {
	config := defaultConfig()
	return provider{config: config, options: append([]Option(nil), options...)}
}

type provider struct {
	runaprovider.Base
	config  Config
	options []Option
}

func (provider provider) Name() string { return "devtools" }
func (item provider) Register(ctx runaprovider.Context) error {
	config := item.config
	if err := runaconfig.BindProvider(ctx, "devtools", "", &config); err != nil {
		return err
	}
	for _, option := range item.options {
		if option != nil {
			option(&config)
		}
	}
	return ctx.RegisterCommand(embedViewCommand{config: config}, buildCommand{}, newCommand{})
}
