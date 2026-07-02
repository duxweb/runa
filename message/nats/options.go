package nats

import (
	"time"

	natsgo "github.com/nats-io/nats.go"
)

type Option func(*options)

type options struct {
	prefix       string
	drainTimeout time.Duration
	driverName   string
	configPath   string
	useName      string
	conn         *natsgo.Conn
	url          string
}

func defaultOptions() options {
	return options{prefix: "runa.message.", drainTimeout: 2 * time.Second, driverName: defaultDriverName, useName: defaultSharedName, url: defaultNATSURL}
}

func Prefix(value string) Option {
	return func(options *options) { options.prefix = value }
}

func DrainTimeout(value time.Duration) Option {
	return func(options *options) {
		if value > 0 {
			options.drainTimeout = value
		}
	}
}

// Conn uses an existing NATS connection. The driver will not close injected connections.
func Conn(conn *natsgo.Conn) Option { return func(options *options) { options.conn = conn } }

// URL sets the NATS URL used when the driver creates its own connection.
func URL(value string) Option { return func(options *options) { options.url = value } }

// Config sets the feature-specific config path. Defaults to message.nats.
func Config(path string) Option { return func(options *options) { options.configPath = path } }

// Use selects the shared nats config name used by Provider.
func Use(name string) Option { return func(options *options) { options.useName = name } }

// Name sets the message driver registration name used by Provider.
func Name(value string) Option { return func(options *options) { options.driverName = value } }
