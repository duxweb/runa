package nats

import (
	"time"

	"github.com/duxweb/runa/config"
	"github.com/duxweb/runa/message"
	runaprovider "github.com/duxweb/runa/provider"
	natsgo "github.com/nats-io/nats.go"
)

const (
	defaultDriverName = "nats"
	defaultSharedName = "default"
	defaultConfigPath = "message.nats"
	defaultNATSURL    = "nats://127.0.0.1:4222"
)

type provider struct {
	runaprovider.Base
	items []Option
}

func Provider(items ...Option) runaprovider.Provider {
	return &provider{items: append([]Option(nil), items...)}
}
func (provider *provider) Name() string  { return "message.nats" }
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
		conn, err = natsgo.Connect(opts.url)
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
		applyNATSConfig(&opts, readNATSConfig(store, sharedNATSPath(selector.useName)))
		path := selector.configPath
		if path == "" {
			path = defaultConfigPath
		}
		applyNATSConfig(&opts, readNATSConfig(store, path))
	}
	applyOptions(&opts, provider.items...)
	normalizeOptions(&opts)
	return opts
}

type natsConfig struct {
	URL          *string        `toml:"url"`
	Prefix       *string        `toml:"prefix"`
	DrainTimeout *time.Duration `toml:"drain_timeout"`
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
		opts.url = defaultNATSURL
	}
	if opts.prefix == "" {
		opts.prefix = "runa.message."
	}
	if opts.drainTimeout <= 0 {
		opts.drainTimeout = 2 * time.Second
	}
}

func readNATSConfig(store *config.Store, path string) natsConfig {
	var item natsConfig
	if store == nil || path == "" || !store.Has(path) {
		return item
	}
	_ = store.Bind(path, &item)
	return item
}

func applyNATSConfig(opts *options, item natsConfig) {
	if item.URL != nil {
		opts.url = *item.URL
	}
	if item.Prefix != nil {
		opts.prefix = *item.Prefix
	}
	if item.DrainTimeout != nil {
		opts.drainTimeout = *item.DrainTimeout
	}
}

func sharedNATSPath(name string) string {
	if name == "" || name == defaultSharedName {
		return "nats"
	}
	return "nats." + name
}

var _ runaprovider.Provider = (*provider)(nil)
