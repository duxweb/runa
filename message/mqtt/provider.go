package mqtt

import (
	"context"
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/message"
	runaprovider "github.com/duxweb/runa/provider"
	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	defaultDriverName = "mqtt"
	defaultSharedName = "default"
	defaultConfigPath = "message.mqtt"
	defaultMQTTBroker = "tcp://127.0.0.1:1883"
)

type provider struct {
	runaprovider.Base
	items []Option
}

func Provider(items ...Option) runaprovider.Provider {
	return &provider{items: append([]Option(nil), items...)}
}
func (provider *provider) Name() string  { return "message.mqtt" }
func (provider *provider) Priority() int { return 10 }

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*message.Registry](ctx)
	if err != nil {
		return err
	}
	opts := provider.resolve(ctx)
	client := opts.client
	ownsClient := false
	if client == nil {
		client, err = newClient(opts)
		if err != nil {
			return err
		}
		ownsClient = true
	}
	driver := newDriver(client, opts, ownsClient)
	registry.RegisterDriver(opts.driverName, driver)
	return nil
}

func (provider *provider) resolve(ctx runaprovider.Context) options {
	opts := defaultOptions()
	selector := opts
	applyOptions(&selector, provider.items...)
	store, _ := runaprovider.Invoke[*config.Store](ctx)
	if store != nil {
		applyMQTTConfig(&opts, readMQTTConfig(store, sharedMQTTPath(selector.useName)))
		path := selector.configPath
		if path == "" {
			path = defaultConfigPath
		}
		applyMQTTConfig(&opts, readMQTTConfig(store, path))
	}
	applyOptions(&opts, provider.items...)
	normalizeOptions(&opts)
	return opts
}

type mqttConfig struct {
	Broker     *string        `toml:"broker"`
	URL        *string        `toml:"url"`
	ClientID   *string        `toml:"client_id"`
	Username   *string        `toml:"username"`
	Password   *string        `toml:"password"`
	Prefix     *string        `toml:"prefix"`
	QoS        *byte          `toml:"qos"`
	Retained   *bool          `toml:"retained"`
	Timeout    *time.Duration `toml:"timeout"`
	Disconnect *uint          `toml:"disconnect"`
}

func applyOptions(opts *options, items ...Option) {
	for _, item := range items {
		if item != nil {
			item(opts)
		}
	}
}

func normalizeOptions(opts *options) {
	if opts.driverName == "" {
		opts.driverName = defaultDriverName
	}
	if opts.useName == "" {
		opts.useName = defaultSharedName
	}
	if opts.broker == "" {
		opts.broker = defaultMQTTBroker
	}
	if opts.prefix == "" {
		opts.prefix = "runa/message/"
	}
	if opts.timeout <= 0 {
		opts.timeout = 5 * time.Second
	}
	if opts.disconnect == 0 {
		opts.disconnect = 250
	}
}

func readMQTTConfig(store *config.Store, path string) mqttConfig {
	var item mqttConfig
	if store == nil || path == "" || !store.Has(path) {
		return item
	}
	_ = store.Bind(path, &item)
	return item
}

func applyMQTTConfig(opts *options, item mqttConfig) {
	if item.Broker != nil {
		opts.broker = *item.Broker
	}
	if item.URL != nil {
		opts.broker = *item.URL
	}
	if item.ClientID != nil {
		opts.clientID = *item.ClientID
	}
	if item.Username != nil {
		opts.username = *item.Username
	}
	if item.Password != nil {
		opts.password = *item.Password
	}
	if item.Prefix != nil {
		opts.prefix = *item.Prefix
	}
	if item.QoS != nil {
		opts.qos = *item.QoS
	}
	if item.Retained != nil {
		opts.retained = *item.Retained
	}
	if item.Timeout != nil {
		opts.timeout = *item.Timeout
	}
	if item.Disconnect != nil {
		opts.disconnect = *item.Disconnect
	}
}

func sharedMQTTPath(name string) string {
	if name == "" || name == defaultSharedName {
		return "mqtt"
	}
	return "mqtt." + name
}

func newClient(opts options) (paho.Client, error) {
	clientOptions := paho.NewClientOptions().AddBroker(opts.broker)
	if opts.clientID != "" {
		clientOptions.SetClientID(opts.clientID)
	}
	if opts.username != "" {
		clientOptions.SetUsername(opts.username)
	}
	if opts.password != "" {
		clientOptions.SetPassword(opts.password)
	}
	client := paho.NewClient(clientOptions)
	if err := wait(context.Background(), client.Connect(), opts.timeout); err != nil {
		return nil, err
	}
	return client, nil
}

var _ runaprovider.Provider = (*provider)(nil)
