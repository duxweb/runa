package mqtt

import (
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

type Option func(*options)

type options struct {
	prefix     string
	qos        byte
	retained   bool
	timeout    time.Duration
	disconnect uint
	driverName string
	configPath string
	useName    string
	client     paho.Client
	broker     string
	clientID   string
	username   string
	password   string
}

func defaultOptions() options {
	return options{prefix: "runa/message/", timeout: 5 * time.Second, disconnect: 250, driverName: defaultDriverName, useName: defaultSharedName, broker: defaultMQTTBroker}
}

func Prefix(value string) Option { return func(options *options) { options.prefix = value } }

func QoS(value byte) Option {
	return func(options *options) {
		if value <= 2 {
			options.qos = value
		}
	}
}

func Retained(value bool) Option { return func(options *options) { options.retained = value } }

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

// Client uses an existing MQTT client. The driver will not disconnect injected clients.
func Client(client paho.Client) Option { return func(options *options) { options.client = client } }

// Broker sets the MQTT broker URL used when the driver creates its own client.
func Broker(value string) Option { return func(options *options) { options.broker = value } }

// URL aliases Broker.
func URL(value string) Option { return Broker(value) }

// ClientID sets the MQTT client id used when the driver creates its own client.
func ClientID(value string) Option { return func(options *options) { options.clientID = value } }

// Credentials sets MQTT username and password.
func Credentials(username string, password string) Option {
	return func(options *options) { options.username, options.password = username, password }
}

// Config sets the feature-specific config path. Defaults to message.mqtt.
func Config(path string) Option { return func(options *options) { options.configPath = path } }

// Use selects the shared mqtt config name used by Provider.
func Use(name string) Option { return func(options *options) { options.useName = name } }

// Name sets the message driver registration name used by Provider.
func Name(value string) Option { return func(options *options) { options.driverName = value } }
