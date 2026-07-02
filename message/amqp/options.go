package amqp

import amqp091 "github.com/rabbitmq/amqp091-go"

type Option func(*options)

type options struct {
	exchange   string
	prefix     string
	driverName string
	configPath string
	useName    string
	conn       *amqp091.Connection
	url        string
}

func defaultOptions() options {
	return options{exchange: "runa.message", driverName: defaultDriverName, useName: defaultSharedName, url: defaultAMQPURL}
}

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

// Conn uses an existing AMQP connection. The driver will not close injected connections.
func Conn(conn *amqp091.Connection) Option { return func(options *options) { options.conn = conn } }

// URL sets the AMQP URL used when the driver creates its own connection.
func URL(value string) Option { return func(options *options) { options.url = value } }

// Config sets the feature-specific config path. Defaults to message.amqp.
func Config(path string) Option { return func(options *options) { options.configPath = path } }

// Use selects the shared amqp config name used by Provider.
func Use(name string) Option { return func(options *options) { options.useName = name } }

// Name sets the message driver registration name used by Provider.
func Name(value string) Option { return func(options *options) { options.driverName = value } }
