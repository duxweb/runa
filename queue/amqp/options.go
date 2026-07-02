package amqp

import amqp091 "github.com/rabbitmq/amqp091-go"

// Option configures AMQP queue driver.
type Option func(*options)

type options struct {
	exchange   string
	prefix     string
	prefetch   int
	driverName string
	configPath string
	useName    string
	conn       *amqp091.Connection
	url        string
}

// Conn uses an existing AMQP connection. The driver will not close injected connections.
func Conn(conn *amqp091.Connection) Option { return func(options *options) { options.conn = conn } }

// URL sets the AMQP URL used when the driver creates its own connection.
func URL(value string) Option { return func(options *options) { options.url = value } }

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

// Config sets the feature-specific config path. Defaults to queue.amqp.
func Config(path string) Option { return func(options *options) { options.configPath = path } }

// Use selects the shared amqp config name used by Provider.
func Use(name string) Option { return func(options *options) { options.useName = name } }

// Name sets the queue driver registration name used by Provider.
func Name(value string) Option { return func(options *options) { options.driverName = value } }
