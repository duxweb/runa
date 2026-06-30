package cache

import (
	"time"

	"github.com/duxweb/runa/core"
)

// Option configures a named cache pool.
type Option interface {
	ApplyCache(*Options)
}

// DriverOption configures a cache driver.
type DriverOption interface {
	ApplyDriver(*DriverOptions)
}

// Options stores cache pool settings.
type Options struct {
	Driver string
	Prefix string
	Codec  Serializer
	TTL    time.Duration
	Jitter JitterConfig
	Meta   core.Map
}

// DriverOptions stores driver settings.
type DriverOptions struct {
	Name     string
	Prefix   string
	Codec    Serializer
	TTL      time.Duration
	Jitter   JitterConfig
	Capacity int
	Meta     core.Map
}

// Jitter randomizes TTL values.
type JitterConfig struct {
	Lambda     float64
	UpperBound time.Duration
}

type optionFunc func(*Options)

func (fn optionFunc) ApplyCache(options *Options) { fn(options) }

type driverOptionFunc func(*DriverOptions)

func (fn driverOptionFunc) ApplyDriver(options *DriverOptions) { fn(options) }

// Use selects the backing driver used by a cache pool.
func Use(name string) Option {
	return optionFunc(func(options *Options) { options.Driver = name })
}

// Prefix sets a key prefix.
func Prefix(value string) prefixOption { return prefixOption{value: value} }

type prefixOption struct{ value string }

func (option prefixOption) ApplyCache(options *Options)        { options.Prefix = option.value }
func (option prefixOption) ApplyDriver(options *DriverOptions) { options.Prefix = option.value }

// Codec sets the serializer.
func Codec(serializer Serializer) codecOption { return codecOption{codec: serializer} }

type codecOption struct{ codec Serializer }

func (option codecOption) ApplyCache(options *Options)        { options.Codec = option.codec }
func (option codecOption) ApplyDriver(options *DriverOptions) { options.Codec = option.codec }

// TTL sets the default ttl.
func TTL(duration time.Duration) ttlOption { return ttlOption{duration: duration} }

type ttlOption struct{ duration time.Duration }

func (option ttlOption) ApplyCache(options *Options)        { options.TTL = option.duration }
func (option ttlOption) ApplyDriver(options *DriverOptions) { options.TTL = option.duration }

// Jitter sets TTL jitter.
func Jitter(lambda float64, upperBound time.Duration) jitterOption {
	return jitterOption{jitter: JitterConfig{Lambda: lambda, UpperBound: upperBound}}
}

type jitterOption struct{ jitter JitterConfig }

func (option jitterOption) ApplyCache(options *Options)        { options.Jitter = option.jitter }
func (option jitterOption) ApplyDriver(options *DriverOptions) { options.Jitter = option.jitter }

// Capacity sets memory driver capacity in bytes.
func Capacity(value int) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) { options.Capacity = value })
}

// DriverMeta sets store metadata.
func DriverMeta(key string, value any) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) {
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	})
}

// Meta sets cache pool metadata.
func Meta(key string, value any) Option {
	return optionFunc(func(options *Options) {
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	})
}

func normalizeOptions(options Options) Options {
	if options.Driver == "" {
		options.Driver = DefaultDriver
	}
	if options.Codec == nil {
		options.Codec = JSONCodec()
	}
	if options.TTL <= 0 {
		options.TTL = DefaultTTL
	}
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	return options
}

func normalizeDriverOptions(options DriverOptions) DriverOptions {
	if options.Codec == nil {
		options.Codec = JSONCodec()
	}
	if options.TTL <= 0 {
		options.TTL = DefaultTTL
	}
	if options.Capacity <= 0 {
		options.Capacity = DefaultCapacity
	}
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	return options
}
