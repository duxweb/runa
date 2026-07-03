package s3

import (
	"github.com/duxweb/runa/config"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/storage"
)

const (
	defaultDriverName = "s3"
	defaultSharedName = "default"
	defaultConfigPath = "storage.s3"
)

type provider struct {
	runaprovider.Base
	items []Option
}

// Provider registers an S3-compatible storage driver.
func Provider(items ...Option) runaprovider.Provider {
	return &provider{items: append([]Option(nil), items...)}
}

func (provider *provider) Name() string { return "storage.s3" }

func (provider *provider) Priority() int { return 10 }

func (provider *provider) Register(ctx runaprovider.Context) error {
	registry, err := runaprovider.Invoke[*storage.Registry](ctx)
	if err != nil {
		return err
	}
	opts := provider.resolve(ctx)
	registry.RegisterDriver(opts.name, newDriver(opts))
	return nil
}

func (provider *provider) resolve(ctx runaprovider.Context) options {
	opts := defaultOptions()
	selector := opts
	applyOptions(&selector, provider.items...)
	store, _ := runaprovider.Invoke[*config.Store](ctx)
	if store != nil {
		applyS3ConnectionConfig(&opts, readS3Config(store, sharedS3Path(selector.useName)))
		path := selector.configPath
		if path == "" {
			path = defaultConfigPath
		}
		applyS3Config(&opts, readS3Config(store, path))
	}
	applyOptions(&opts, provider.items...)
	normalizeOptions(&opts)
	return opts
}

type s3Config struct {
	Name         *string `toml:"name"`
	Bucket       *string `toml:"bucket"`
	Region       *string `toml:"region"`
	Endpoint     *string `toml:"endpoint"`
	AccessKey    *string `toml:"access_key"`
	SecretKey    *string `toml:"secret_key"`
	SessionToken *string `toml:"session_token"`
	Domain       *string `toml:"domain"`
	URLPrefix    *string `toml:"url_prefix"`
	PathStyle    *bool   `toml:"path_style"`
}

func applyOptions(opts *options, items ...Option) {
	for _, item := range items {
		if item != nil {
			item(opts)
		}
	}
}

func normalizeOptions(opts *options) {
	if opts.name == "" {
		opts.name = defaultDriverName
	}
	if opts.useName == "" {
		opts.useName = defaultSharedName
	}
}

func readS3Config(store *config.Store, path string) s3Config {
	var item s3Config
	if store == nil || path == "" || !store.Has(path) {
		return item
	}
	_ = store.Bind(path, &item)
	return item
}

func applyS3ConnectionConfig(opts *options, item s3Config) {
	if item.Bucket != nil {
		opts.bucket = *item.Bucket
	}
	if item.Region != nil {
		opts.region = *item.Region
	}
	if item.Endpoint != nil {
		opts.endpoint = *item.Endpoint
	}
	if item.AccessKey != nil {
		opts.accessKey = *item.AccessKey
	}
	if item.SecretKey != nil {
		opts.secretKey = *item.SecretKey
	}
	if item.SessionToken != nil {
		opts.sessionToken = *item.SessionToken
	}
	if item.PathStyle != nil {
		opts.pathStyle = *item.PathStyle
	}
}

func applyS3Config(opts *options, item s3Config) {
	applyS3ConnectionConfig(opts, item)
	if item.Name != nil {
		opts.name = *item.Name
	}
	if item.Domain != nil {
		opts.domain = *item.Domain
	}
	if item.URLPrefix != nil {
		opts.urlPrefix = *item.URLPrefix
	}
}

func sharedS3Path(name string) string {
	if name == "" || name == defaultSharedName {
		return "s3"
	}
	return "s3." + name
}

var _ runaprovider.Provider = (*provider)(nil)
