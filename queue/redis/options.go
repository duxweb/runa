package redis

import "time"

// Option configures Redis queue driver.
type Option func(*options)

type options struct {
	prefix string
	now    func() time.Time
}

// Prefix sets Redis key prefix.
func Prefix(value string) Option {
	return func(options *options) {
		if value != "" {
			options.prefix = value
		}
	}
}
