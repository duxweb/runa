package lock

import (
	"time"

	"github.com/duxweb/runa/core"
)

// LockerOption configures a named locker.
type LockerOption interface {
	ApplyLocker(*Options)
}

// LockOption configures one lock acquisition.
type LockOption interface {
	ApplyLock(*Options)
}

// DriverOption configures a lock driver.
type DriverOption interface {
	ApplyDriver(*DriverOptions)
}

// Options stores locker and lock-call settings.
type Options struct {
	Driver        string
	Prefix        string
	TTL           time.Duration
	Wait          time.Duration
	RetryInterval time.Duration
	AutoRenew     bool
	Meta          core.Map
}

// DriverOptions stores backend lock driver settings.
type DriverOptions struct {
	Name   string
	Prefix string
	Meta   core.Map
}

type lockerOptionFunc func(*Options)

func (fn lockerOptionFunc) ApplyLocker(options *Options) { fn(options) }

type lockOptionFunc func(*Options)

func (fn lockOptionFunc) ApplyLock(options *Options) { fn(options) }

type driverOptionFunc func(*DriverOptions)

func (fn driverOptionFunc) ApplyDriver(options *DriverOptions) { fn(options) }

// Driver selects the backing lock driver used by a locker.
func Use(name string) LockerOption {
	return lockerOptionFunc(func(options *Options) { options.Driver = name })
}

// Prefix sets a key prefix for a locker or store.
func Prefix(value string) prefixOption { return prefixOption{value: value} }

type prefixOption struct{ value string }

func (option prefixOption) ApplyLocker(options *Options)       { options.Prefix = option.value }
func (option prefixOption) ApplyDriver(options *DriverOptions) { options.Prefix = option.value }

// TTL sets lock ttl.
func TTL(duration time.Duration) ttlOption { return ttlOption{duration: duration} }

type ttlOption struct{ duration time.Duration }

func (option ttlOption) ApplyLocker(options *Options) { options.TTL = option.duration }
func (option ttlOption) ApplyLock(options *Options)   { options.TTL = option.duration }

// Wait sets max blocking wait duration.
func Wait(duration time.Duration) waitOption { return waitOption{duration: duration} }

type waitOption struct{ duration time.Duration }

func (option waitOption) ApplyLocker(options *Options) { options.Wait = option.duration }
func (option waitOption) ApplyLock(options *Options)   { options.Wait = option.duration }

// RetryInterval sets polling interval while waiting.
func RetryInterval(duration time.Duration) retryIntervalOption {
	return retryIntervalOption{duration: duration}
}

type retryIntervalOption struct{ duration time.Duration }

func (option retryIntervalOption) ApplyLocker(options *Options) {
	options.RetryInterval = option.duration
}
func (option retryIntervalOption) ApplyLock(options *Options) {
	options.RetryInterval = option.duration
}

// AutoRenew enables or disables automatic lease renew in With.
func AutoRenew(enabled bool) autoRenewOption { return autoRenewOption{enabled: enabled} }

type autoRenewOption struct{ enabled bool }

func (option autoRenewOption) ApplyLocker(options *Options) { options.AutoRenew = option.enabled }
func (option autoRenewOption) ApplyLock(options *Options)   { options.AutoRenew = option.enabled }

// Meta sets locker metadata.
func Meta(key string, value any) metaOption { return metaOption{key: key, value: value} }

type metaOption struct {
	key   string
	value any
}

func (option metaOption) ApplyLocker(options *Options) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}

func (option metaOption) ApplyLock(options *Options) {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	options.Meta[option.key] = option.value
}

// DriverMeta sets lock driver metadata.
func DriverMeta(key string, value any) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) {
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
	if options.TTL <= 0 {
		options.TTL = DefaultTTL
	}
	if options.Wait <= 0 {
		options.Wait = DefaultWait
	}
	if options.RetryInterval <= 0 {
		options.RetryInterval = DefaultRetryInterval
	}
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	return options
}

func normalizeDriverOptions(options DriverOptions) DriverOptions {
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	return options
}
