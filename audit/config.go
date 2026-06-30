package audit

import (
	"context"
	"net/http"
	"time"
)

// Mode controls audit writing behavior.
type Mode string

const (
	Sync  Mode = "sync"
	Async Mode = "async"
)

// Config configures audit middleware.
type Config struct {
	Methods []string `toml:"methods"`
	Mode    Mode     `toml:"mode"`
	Strict  bool     `toml:"strict"`

	Write  func(ctx context.Context, entry Entry) error `toml:"-"`
	Writer Writer                                       `toml:"-"`

	CaptureInput bool     `toml:"capture_input"`
	MaskFields   []string `toml:"mask_fields"`
	MaskValue    string   `toml:"mask_value"`
	MaxInputSize int      `toml:"max_input_size"`

	Buffer       int           `toml:"buffer"`
	WriteTimeout time.Duration `toml:"write_timeout"`
}

// Normalize fills audit config defaults.
func Normalize(config Config) Config {
	if len(config.Methods) == 0 {
		config.Methods = []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	}
	if config.Mode == "" {
		config.Mode = Async
	}
	if config.MaskValue == "" {
		config.MaskValue = "***"
	}
	if config.MaxInputSize <= 0 {
		config.MaxInputSize = 16 << 10
	}
	if config.WriteTimeout <= 0 {
		config.WriteTimeout = 3 * time.Second
	}
	if len(config.MaskFields) == 0 {
		config.MaskFields = DefaultMaskFields()
	}
	return config
}

func normalize(config Config) Config { return Normalize(config) }
