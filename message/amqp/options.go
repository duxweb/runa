package amqp

type Option func(*options)

type options struct {
	exchange string
	prefix   string
}

func defaultOptions() options { return options{exchange: "runa.message"} }

func Exchange(value string) Option {
	return func(options *options) {
		if value != "" {
			options.exchange = value
		}
	}
}

func Prefix(value string) Option {
	return func(options *options) { options.prefix = value }
}
