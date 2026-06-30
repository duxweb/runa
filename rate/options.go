package rate

import (
	"time"

	"github.com/duxweb/runa/core"
)

// Option configures a named limiter.
type Option interface{ ApplyRate(*Rule) }

// DriverOption configures driver stores.
type DriverOption interface{ ApplyDriver(*DriverOptions) }

type optionFunc func(*Rule)
type driverOptionFunc func(*DriverOptions)

func (fn optionFunc) ApplyRate(rule *Rule)                  { fn(rule) }
func (fn driverOptionFunc) ApplyDriver(opts *DriverOptions) { fn(opts) }

// DriverOptions stores driver metadata.
type DriverOptions struct {
	Name   string
	Prefix string
	Meta   core.Map
}

// Driver selects the rate driver.
func Use(name string) Option {
	return optionFunc(func(rule *Rule) { rule.Driver = name })
}

// TokenBucket configures token bucket limit.
func TokenBucket(limit int, window time.Duration) Option {
	return optionFunc(func(rule *Rule) {
		rule.Algorithm = AlgorithmTokenBucket
		rule.Limit = limit
		rule.Window = window
		rule.Burst = limit
	})
}

// FixedWindowRule configures fixed window limit.
func FixedWindow(limit int, window time.Duration) Option {
	return optionFunc(func(rule *Rule) {
		rule.Algorithm = AlgorithmFixedWindow
		rule.Limit = limit
		rule.Window = window
		rule.Burst = limit
	})
}

// SlidingWindowRule configures sliding window limit.
func SlidingWindow(limit int, window time.Duration) Option {
	return optionFunc(func(rule *Rule) {
		rule.Algorithm = AlgorithmSlidingWindow
		rule.Limit = limit
		rule.Window = window
		rule.Burst = limit
	})
}

// Burst sets token bucket burst size.
func Burst(value int) Option {
	return optionFunc(func(rule *Rule) { rule.Burst = value })
}

// Key sets rate key sources.
func Key(sources ...KeySource) Option {
	return optionFunc(func(rule *Rule) { rule.Key = append([]KeySource(nil), sources...) })
}

// Meta stores arbitrary limiter metadata.
func Meta(key string, value any) Option {
	return optionFunc(func(rule *Rule) {
		if rule.Meta == nil {
			rule.Meta = make(core.Map)
		}
		rule.Meta[key] = value
	})
}

// Name sets store name metadata.
func Name(value string) DriverOption {
	return driverOptionFunc(func(opts *DriverOptions) { opts.Name = value })
}

// Prefix sets store key prefix.
func Prefix(value string) DriverOption {
	return driverOptionFunc(func(opts *DriverOptions) { opts.Prefix = value })
}

// DriverMeta stores arbitrary driver metadata.
func DriverMeta(key string, value any) DriverOption {
	return driverOptionFunc(func(opts *DriverOptions) {
		if opts.Meta == nil {
			opts.Meta = make(core.Map)
		}
		opts.Meta[key] = value
	})
}

func applyRule(name string, options ...Option) Rule {
	rule := Rule{Name: name, Driver: DefaultDriver, Algorithm: AlgorithmTokenBucket, Limit: 60, Window: time.Minute, Burst: 60, Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyRate(&rule)
		}
	}
	if rule.Driver == "" {
		rule.Driver = DefaultDriver
	}
	if rule.Algorithm == "" {
		rule.Algorithm = AlgorithmTokenBucket
	}
	if rule.Limit <= 0 {
		rule.Limit = 60
	}
	if rule.Window <= 0 {
		rule.Window = time.Minute
	}
	if rule.Burst <= 0 {
		rule.Burst = rule.Limit
	}
	return rule
}

func applyDriverOptions(options ...DriverOption) DriverOptions {
	opts := DriverOptions{Name: DefaultDriver, Prefix: "runa:rate:", Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyDriver(&opts)
		}
	}
	if opts.Name == "" {
		opts.Name = DefaultDriver
	}
	if opts.Prefix == "" {
		opts.Prefix = "runa:rate:"
	}
	return opts
}
