package redis

type Option func(*options)

type options struct {
	prefix string
}

func defaultOptions() options { return options{prefix: "runa:message:"} }

func Prefix(value string) Option {
	return func(options *options) {
		options.prefix = value
	}
}
