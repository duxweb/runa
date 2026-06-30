package database

type ProviderOption func(*provider)

// RegisterDriver registers a database driver with the provider.
func RegisterDriver(name string, driver Driver) ProviderOption {
	return func(provider *provider) {
		if name == "" {
			name = DefaultName
		}
		if driver != nil {
			provider.drivers[name] = driver
		}
	}
}
