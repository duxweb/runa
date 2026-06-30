package nats

import "time"

type Option func(*options)

type options struct {
	prefix       string
	drainTimeout time.Duration
}

func defaultOptions() options { return options{prefix: "runa.message.", drainTimeout: 2 * time.Second} }

func Prefix(value string) Option {
	return func(options *options) { options.prefix = value }
}

func DrainTimeout(value time.Duration) Option {
	return func(options *options) {
		if value > 0 {
			options.drainTimeout = value
		}
	}
}
