package audit

import (
	runaconfig "github.com/duxweb/runa/config"
	runaprovider "github.com/duxweb/runa/provider"
)

// Provider registers audit config through Runa lifecycle.
func Provider(config Config) runaprovider.Provider {
	return provider{config: config}
}

type provider struct {
	runaprovider.Base
	config Config
}

func (provider provider) Name() string { return "audit" }
func (provider provider) Register(ctx runaprovider.Context) error {
	config := normalize(Config{})
	if err := runaconfig.BindProvider(ctx, "audit", "", &config); err != nil {
		return err
	}
	applyCodeConfig(&config, provider.config)
	runaprovider.ProvideValueOnce(ctx, New(config))
	return nil
}

func applyCodeConfig(config *Config, code Config) {
	if len(code.Methods) > 0 {
		config.Methods = append([]string(nil), code.Methods...)
	}
	if code.Mode != "" {
		config.Mode = code.Mode
	}
	if code.Strict {
		config.Strict = true
	}
	if code.Write != nil {
		config.Write = code.Write
	}
	if code.Writer != nil {
		config.Writer = code.Writer
	}
	if code.CaptureInput {
		config.CaptureInput = true
	}
	if len(code.MaskFields) > 0 {
		config.MaskFields = append([]string(nil), code.MaskFields...)
	}
	if code.MaskValue != "" {
		config.MaskValue = code.MaskValue
	}
	if code.MaxInputSize > 0 {
		config.MaxInputSize = code.MaxInputSize
	}
	if code.Buffer > 0 {
		config.Buffer = code.Buffer
	}
	if code.WriteTimeout > 0 {
		config.WriteTimeout = code.WriteTimeout
	}
}
