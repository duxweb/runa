package config

import runaprovider "github.com/duxweb/runa/provider"

// BindProvider binds a provider-scoped config subtree into target.
func BindProvider(ctx runaprovider.Context, scope string, key string, target any) error {
	store, err := runaprovider.Invoke[*Store](ctx)
	if err != nil {
		return err
	}
	return store.Scope(scope).Bind(key, target)
}
