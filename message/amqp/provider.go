package amqp

import (
	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/message"
	runaprovider "github.com/duxweb/runa/provider"
	amqp091 "github.com/rabbitmq/amqp091-go"
)

const (
	defaultDriverName = "amqp"
	defaultSharedName = "default"
	defaultConfigPath = "message.amqp"
	defaultAMQPURL    = "amqp://guest:guest@127.0.0.1:5672/"
)

type provider struct {
	runaprovider.Base
	items []Option
}

func Provider(items ...Option) runaprovider.Provider {
	return &provider{items: append([]Option(nil), items...)}
}
func (provider *provider) Name() string  { return "message.amqp" }
func (provider *provider) Priority() int { return 10 }

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*message.Registry](ctx)
	if err != nil {
		return err
	}
	opts := provider.resolve(ctx)
	conn := opts.conn
	ownsConn := false
	if conn == nil {
		conn, err = amqp091.Dial(opts.url)
		if err != nil {
			return err
		}
		ownsConn = true
	}
	driver := newDriver(conn, opts, ownsConn)
	registry.RegisterDriver(opts.driverName, driver)
	return nil
}

func (provider *provider) resolve(ctx runaprovider.Context) options {
	opts := defaultOptions()
	selector := opts
	applyOptions(&selector, provider.items...)
	store, _ := runaprovider.Invoke[*config.Store](ctx)
	if store != nil {
		applyAMQPConfig(&opts, readAMQPConfig(store, sharedAMQPPath(selector.useName)))
		path := selector.configPath
		if path == "" {
			path = defaultConfigPath
		}
		applyAMQPConfig(&opts, readAMQPConfig(store, path))
	}
	applyOptions(&opts, provider.items...)
	normalizeOptions(&opts)
	return opts
}

type amqpConfig struct {
	URL      *string `toml:"url"`
	Exchange *string `toml:"exchange"`
	Prefix   *string `toml:"prefix"`
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
	if opts.url == "" {
		opts.url = defaultAMQPURL
	}
	if opts.exchange == "" {
		opts.exchange = "runa.message"
	}
}

func readAMQPConfig(store *config.Store, path string) amqpConfig {
	var item amqpConfig
	if store == nil || path == "" || !store.Has(path) {
		return item
	}
	_ = store.Bind(path, &item)
	return item
}

func applyAMQPConfig(opts *options, item amqpConfig) {
	if item.URL != nil {
		opts.url = *item.URL
	}
	if item.Exchange != nil {
		opts.exchange = *item.Exchange
	}
	if item.Prefix != nil {
		opts.prefix = *item.Prefix
	}
}

func sharedAMQPPath(name string) string {
	if name == "" || name == defaultSharedName {
		return "amqp"
	}
	return "amqp." + name
}

var _ runaprovider.Provider = (*provider)(nil)
