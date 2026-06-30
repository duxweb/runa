package helmet

import "github.com/duxweb/runa/route"

// Config configures security headers middleware.
type Config struct {
	Next                    func(*route.Context) bool
	ContentTypeNosniff      string
	FrameOptions            string
	ReferrerPolicy          string
	XSSProtection           string
	CrossOriginOpenerPolicy string
	Custom                  map[string]string
}

// New creates security headers middleware.
func New(configs ...Config) route.Middleware {
	config := firstConfig(configs...)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			header := ctx.Response().Header()
			set(header.Set, "X-Content-Type-Options", config.ContentTypeNosniff)
			set(header.Set, "X-Frame-Options", config.FrameOptions)
			set(header.Set, "Referrer-Policy", config.ReferrerPolicy)
			set(header.Set, "X-XSS-Protection", config.XSSProtection)
			set(header.Set, "Cross-Origin-Opener-Policy", config.CrossOriginOpenerPolicy)
			for key, value := range config.Custom {
				set(header.Set, key, value)
			}
			return next(ctx)
		}
	}
}

func firstConfig(configs ...Config) Config {
	config := Config{
		ContentTypeNosniff:      "nosniff",
		FrameOptions:            "SAMEORIGIN",
		ReferrerPolicy:          "no-referrer-when-downgrade",
		XSSProtection:           "0",
		CrossOriginOpenerPolicy: "same-origin",
	}
	if len(configs) > 0 {
		provided := configs[0]
		if provided.Next != nil {
			config.Next = provided.Next
		}
		if provided.ContentTypeNosniff != "" {
			config.ContentTypeNosniff = provided.ContentTypeNosniff
		}
		if provided.FrameOptions != "" {
			config.FrameOptions = provided.FrameOptions
		}
		if provided.ReferrerPolicy != "" {
			config.ReferrerPolicy = provided.ReferrerPolicy
		}
		if provided.XSSProtection != "" {
			config.XSSProtection = provided.XSSProtection
		}
		if provided.CrossOriginOpenerPolicy != "" {
			config.CrossOriginOpenerPolicy = provided.CrossOriginOpenerPolicy
		}
		if provided.Custom != nil {
			config.Custom = provided.Custom
		}
	}
	return config
}

func set(fn func(string, string), key string, value string) {
	if value != "" {
		fn(key, value)
	}
}
