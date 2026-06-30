package mqtt

import "time"

type Option func(*options)

type options struct {
	prefix     string
	qos        byte
	retained   bool
	timeout    time.Duration
	disconnect uint
}

func defaultOptions() options {
	return options{prefix: "runa/message/", timeout: 5 * time.Second, disconnect: 250}
}

func Prefix(value string) Option {
	return func(options *options) { options.prefix = value }
}

func QoS(value byte) Option {
	return func(options *options) {
		if value <= 2 {
			options.qos = value
		}
	}
}

func Retained(value bool) Option {
	return func(options *options) { options.retained = value }
}

func Timeout(value time.Duration) Option {
	return func(options *options) {
		if value > 0 {
			options.timeout = value
		}
	}
}

func DisconnectQuiesce(value uint) Option {
	return func(options *options) { options.disconnect = value }
}
