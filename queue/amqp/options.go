package amqp

// Option configures AMQP queue driver.
type Option func(*options)

type options struct {
	exchange string
	prefix   string
	prefetch int
}

// Exchange sets AMQP exchange name.
func Exchange(value string) Option {
	return func(options *options) { options.exchange = value }
}

// Prefix sets queue name prefix.
func Prefix(value string) Option {
	return func(options *options) { options.prefix = value }
}

// Prefetch sets the AMQP channel prefetch count for consumers.
func Prefetch(value int) Option {
	return func(options *options) { options.prefetch = value }
}
